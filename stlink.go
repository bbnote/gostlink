// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

// this code is mainly inspired and based on the openocd project source code
// for detailed information see

// https://sourceforge.net/p/openocd/code

package gostlink

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/gousb"
	log "github.com/sirupsen/logrus"
)

const STLINK_ALL_VIDS = 0xFFFF
const STLINK_ALL_PIDS = 0xFFFF

var stlink_supported_vids = []gousb.ID{0x0483} // STLINK Vendor ID
var stlink_supported_pids = []gousb.ID{0x3744, 0x3748, 0x374b, 0x374d, 0x374e, 0x374f, 0x3752, 0x3753}

type StLinkVersion struct {
	/** */
	stlink int
	/** */
	jtag int
	/** */
	swim int
	/** jtag api version supported */
	jtag_api StLinkApiVersion
	/** one bit for each feature supported. See macros STLINK_F_* */
	flags uint32
}

/** */
type StLinkHandle struct {
	/** */
	usb_device *gousb.Device
	/** */
	usb_config *gousb.Config
	/** */
	usb_interface *gousb.Interface
	/** */
	rx_ep uint8
	/** */
	tx_ep uint8
	/** */
	trace_ep uint8
	/** */
	cmdbuf []byte
	/** */
	cmdidx uint8
	/** */
	direction uint8
	/** */
	databuf []byte
	/** */
	max_mem_packet uint32
	/** */
	st_mode StLinkMode
	/** */
	version StLinkVersion

	/** */
	vid gousb.ID
	/** */
	pid gousb.ID
	/** reconnect is needed next time we try to query the
	 * status */
	reconnect_pending bool
}

type StLinkInterfaceConfig struct {
	vid                 gousb.ID
	pid                 gousb.ID
	mode                StLinkMode
	serial              string
	initial_speed       int
	connect_under_reset bool
}

func NewStLinkConfig(vid gousb.ID, pid gousb.ID, mode StLinkMode,
	serial string, initial_speed int, connect_under_reset bool) *StLinkInterfaceConfig {

	config := &StLinkInterfaceConfig{
		vid:                 vid,
		pid:                 pid,
		mode:                mode,
		serial:              serial,
		initial_speed:       initial_speed,
		connect_under_reset: connect_under_reset,
	}

	return config
}

func NewStLink(config *StLinkInterfaceConfig) (*StLinkHandle, error) {
	var err error
	var devices []*gousb.Device

	handle := &StLinkHandle{}
	handle.st_mode = config.mode

	// initialize data buffers for tx and rx
	handle.cmdbuf = make([]byte, STLINK_SG_SIZE)
	handle.databuf = make([]byte, STLINK_DATA_SIZE)

	if config.vid == STLINK_ALL_VIDS && config.pid == STLINK_ALL_PIDS {
		devices, err = usb_find_devices(stlink_supported_vids, stlink_supported_pids)

	} else if config.vid == STLINK_ALL_VIDS && config.pid != STLINK_ALL_PIDS {
		devices, err = usb_find_devices(stlink_supported_vids, []gousb.ID{config.pid})

	} else if config.vid != STLINK_ALL_VIDS && config.pid == STLINK_ALL_PIDS {
		devices, err = usb_find_devices([]gousb.ID{config.vid}, stlink_supported_pids)

	} else {
		devices, err = usb_find_devices([]gousb.ID{config.vid}, []gousb.ID{config.pid})
	}

	if err != nil {
		return nil, err
	}

	if len(devices) > 0 {

		if config.serial == "" && len(devices) > 1 {
			return nil, errors.New("Could not idendify exact stlink by given paramters. (Perhaps a serial no is missing?)")
		} else if len(devices) == 1 {
			handle.usb_device = devices[0]
		} else {
			for _, dev := range devices {
				dev_serial_no, _ := dev.SerialNumber()

				log.Debugf("Compare serial no %s with number %s", dev_serial_no, config.serial)

				if dev_serial_no == config.serial {
					handle.usb_device = dev

					log.Infof("Found st link with serial number %s", dev_serial_no)
				}
			}
		}
	} else {
		return nil, errors.New("Could not find any ST-Link connected to computer.")
	}

	if handle.usb_device == nil {
		return nil, errors.New("Could not find ST-Link by given paramters")
	}

	// no request required configuration an matching usb interface :D

	handle.usb_config, err = handle.usb_device.Config(1)
	if err != nil {
		log.Debug(err)
		return nil, errors.New("Could not request configuration #0 for st-link debugger.")
	}

	handle.usb_interface, err = handle.usb_config.Interface(0, 0)
	if err != nil {
		log.Debug(err)
		return nil, errors.New("Could not claim interface 0,0 for st-link debugger.")
	}

	handle.rx_ep = STLINK_RX_EP // Endpoint for rx is on all st links the same

	switch handle.usb_device.Desc.Product {
	case STLINK_V1_PID:
		handle.version.stlink = 1
		handle.tx_ep = STLINK_TX_EP

	case STLINK_V3_USBLOADER_PID, STLINK_V3E_PID, STLINK_V3S_PID, STLINK_V3_2VCP_PID:
		handle.version.stlink = 3
		handle.tx_ep = STLINK_V2_1_TX_EP
		handle.trace_ep = STLINK_V2_1_TRACE_EP

	case STLINK_V2_1_PID, STLINK_V2_1_NO_MSD_PID:
		handle.version.stlink = 2
		handle.tx_ep = STLINK_V2_1_TX_EP
		handle.trace_ep = STLINK_V2_1_TRACE_EP

	default:
		log.Infof("Could not determine pid of debugger %04x. Assuming Link V2", handle.usb_device.Desc.Product)
		handle.version.stlink = 2
		handle.tx_ep = STLINK_TX_EP
		handle.trace_ep = STLINK_TRACE_EP
	}

	err = handle.usb_parse_version()

	if err != nil {
		return nil, err
	}

	switch handle.st_mode {
	case STLINK_MODE_DEBUG_SWD:
		if handle.version.jtag_api == STLINK_JTAG_API_V1 {
			return nil, errors.New("SWD not supported by jtag api v1")
		}
	case STLINK_MODE_DEBUG_JTAG:
		if handle.version.jtag == 0 {
			return nil, errors.New("JTAG transport not supported by stlink")
		}
	case STLINK_MODE_DEBUG_SWIM:
		if handle.version.swim == 0 {
			return nil, errors.New("Swim transport not supported by device")
		}

	default:
		return nil, errors.New("Unknown ST-Link mode")
	}

	err = handle.usb_init_mode(config.connect_under_reset, config.initial_speed)

	if err != nil {
		return nil, err
	}

	/** TODO: Implement SWIM mode configuration
	if (h->st_mode == STLINK_MODE_DEBUG_SWIM) {
		err = stlink_swim_enter(h);
		if (err != ERROR_OK) {
			LOG_ERROR("stlink_swim_enter_failed (unable to connect to the target)");
			goto error_open;
		}
		*fd = h;
		h->max_mem_packet = STLINK_DATA_SIZE;
		return ERROR_OK;
	}
	*/

	handle.max_mem_packet = (1 << 10)

	err = handle.usb_init_access_port(0)

	if err != nil {
		return nil, err
	}

	buffer := make([]byte, 4)
	err_code, err := handle.usb_read_mem32(CPUID, 4, buffer)

	if err_code == ERROR_OK {
		var cpuid uint32 = le_to_h_u32(buffer)
		var i uint32 = (cpuid >> 4) & 0xf

		if i == 4 || i == 3 {
			/* Cortex-M3/M4 has 4096 bytes autoincrement range */
			handle.max_mem_packet = (1 << 12)
		}
	}

	log.Debugf("Using TAR autoincrement: %d", handle.max_mem_packet)

	return handle, nil
}

func (h *StLinkHandle) Close() {
	if h.usb_device != nil {
		log.Debugf("Close ST-Link device [%04x:%04x]", uint16(h.vid), uint16(h.pid))

		h.usb_interface.Close()
		h.usb_config.Close()
		h.usb_device.Close()
	}
}

func (h *StLinkHandle) usb_parse_version() error {
	var v, x, y, jtag, swim, msd, bridge byte = 0, 0, 0, 0, 0, 0, 0

	h.usb_init_buffer(h.rx_ep, 6)

	h.cmdbuf[h.cmdidx] = STLINK_GET_VERSION
	h.cmdidx++

	err := h.usb_xfer_noerrcheck(h.databuf, 6)

	if err != nil {
		return err
	}

	version := be_to_h_u16(h.databuf)

	v = byte((version >> 12) & 0x0f)
	x = byte((version >> 6) & 0x3f)
	y = byte(version & 0x3f)

	h.vid = gousb.ID(le_to_h_u16(h.databuf[2:]))
	h.pid = gousb.ID(le_to_h_u16(h.databuf[4:]))

	switch h.pid {
	case STLINK_V2_1_PID, STLINK_V2_1_NO_MSD_PID:
		if (x <= 22 && y == 7) || (x >= 25 && y >= 7 && y <= 12) {
			msd = x
			swim = y
			jtag = 0
		} else {
			jtag = x
			msd = y
			swim = 0
		}

	default:
		jtag = x
		msd = 0
		swim = y
	}

	/* STLINK-V3 requires a specific command */
	if v == 3 && x == 0 && y == 0 {
		h.usb_init_buffer(h.rx_ep, 16)

		h.cmdbuf[h.cmdidx] = STLINK_APIV3_GET_VERSION_EX
		h.cmdidx++

		err := h.usb_xfer_noerrcheck(h.databuf, 12)

		if err != nil {
			return err
		}

		v = h.databuf[0]
		swim = h.databuf[1]
		jtag = h.databuf[2]
		msd = h.databuf[3]
		bridge = h.databuf[4]
		h.vid = gousb.ID(le_to_h_u16(h.databuf[8:]))
		h.pid = gousb.ID(le_to_h_u16(h.databuf[10:]))
	}

	h.version.stlink = int(v)
	h.version.jtag = int(jtag)
	h.version.swim = int(swim)

	var flags uint32 = 0

	switch h.version.stlink {
	case 1:
		/* ST-LINK/V1 from J11 switch to api-v2 (and support SWD) */
		if h.version.jtag >= 11 {
			h.version.jtag_api = STLINK_JTAG_API_V2
		} else {
			h.version.jtag_api = STLINK_JTAG_API_V1
		}
	case 2:
		/* all ST-LINK/V2 and ST-Link/V2.1 use api-v2 */
		h.version.jtag_api = STLINK_JTAG_API_V2

		/* API for trace from J13 */
		/* API for target voltage from J13 */
		if h.version.jtag >= 13 {
			flags |= STLINK_F_HAS_TRACE
		}

		/* preferred API to get last R/W status from J15 */
		if h.version.jtag >= 15 {
			flags |= STLINK_F_HAS_GETLASTRWSTATUS2
		}

		/* API to set SWD frequency from J22 */
		if h.version.jtag >= 22 {
			flags |= STLINK_F_HAS_SWD_SET_FREQ
		}

		/* API to set JTAG frequency from J24 */
		/* API to access DAP registers from J24 */
		if h.version.jtag >= 24 {
			flags |= STLINK_F_HAS_JTAG_SET_FREQ
			flags |= STLINK_F_HAS_DAP_REG
		}

		/* Quirk for read DP in JTAG mode (V2 only) from J24, fixed in J32 */
		if h.version.jtag >= 24 && h.version.jtag < 32 {
			flags |= STLINK_F_QUIRK_JTAG_DP_READ
		}

		/* API to read/write memory at 16 bit from J26 */
		if h.version.jtag >= 26 {
			flags |= STLINK_F_HAS_MEM_16BIT
		}

		/* API required to init AP before any AP access from J28 */
		if h.version.jtag >= 28 {
			flags |= STLINK_F_HAS_AP_INIT
		}

		/* API required to return proper error code on close AP from J29 */
		if h.version.jtag >= 29 {
			flags |= STLINK_F_FIX_CLOSE_AP
		}

		/* Banked regs (DPv1 & DPv2) support from V2J32 */
		if h.version.jtag >= 32 {
			flags |= STLINK_F_HAS_DPBANKSEL
		}
	case 3:
		/* all STLINK-V3 use api-v3 */
		h.version.jtag_api = STLINK_JTAG_API_V3

		/* STLINK-V3 is a superset of ST-LINK/V2 */

		/* API for trace */
		/* API for target voltage */
		flags |= STLINK_F_HAS_TRACE

		/* preferred API to get last R/W status */
		flags |= STLINK_F_HAS_GETLASTRWSTATUS2

		/* API to access DAP registers */
		flags |= STLINK_F_HAS_DAP_REG

		/* API to read/write memory at 16 bit */
		flags |= STLINK_F_HAS_MEM_16BIT

		/* API required to init AP before any AP access */
		flags |= STLINK_F_HAS_AP_INIT

		/* API required to return proper error code on close AP */
		flags |= STLINK_F_FIX_CLOSE_AP

		/* Banked regs (DPv1 & DPv2) support from V3J2 */
		if h.version.jtag >= 2 {
			flags |= STLINK_F_HAS_DPBANKSEL
		}

		/* 8bit read/write max packet size 512 bytes from V3J6 */
		if h.version.jtag >= 6 {
			flags |= STLINK_F_HAS_RW8_512BYTES
		}
	default:
		break
	}

	h.version.flags = flags

	var v_str string = fmt.Sprintf("V%d", v)

	if jtag > 0 || msd != 0 {
		v_str += fmt.Sprintf("J%d", jtag)
	}

	if msd > 0 {
		v_str += fmt.Sprintf("M%d", msd)
	}

	if bridge > 0 {
		v_str += fmt.Sprintf("B%d", bridge)
	}

	serial_no, _ := h.usb_device.SerialNumber()

	log.Debugf("Got ST-Link: %s [%s]", v_str, serial_no)

	return nil
}

func (h *StLinkHandle) GetTargetVoltage() (float32, error) {
	var adc_results [2]uint32

	/* no error message, simply quit with error */
	if (h.version.flags & STLINK_F_HAS_TARGET_VOLT) == 0 {
		return -1.0, errors.New("Device does not support voltage measurement")
	}

	h.usb_init_buffer(h.rx_ep, 8)

	h.cmdbuf[h.cmdidx] = STLINK_GET_TARGET_VOLTAGE
	h.cmdidx++

	err := h.usb_xfer_noerrcheck(h.databuf, 8)

	if err != nil {
		return -1.0, err
	}

	/* convert result */
	adc_results[0] = le_to_h_u32(h.databuf)
	adc_results[1] = le_to_h_u32(h.databuf[4:])

	var target_voltage float32 = 0.0

	if adc_results[0] > 0 {
		target_voltage = 2 * (float32(adc_results[1]) * (1.2 / float32(adc_results[0])))
	}

	log.Infof("Target voltage: %f", target_voltage)

	return target_voltage, nil
}

func (h *StLinkHandle) stlink_speed(khz int, query bool) (int, error) {

	switch h.st_mode {
	/*case STLINK_MODE_DEBUG_SWIM:
	return stlink_speed_swim(khz, query)
	*/

	case STLINK_MODE_DEBUG_SWD:
		if h.version.jtag_api == STLINK_JTAG_API_V3 {
			return h.set_speed_v3(false, khz, query)
		} else {
			return h.set_speed_swd(khz, query)
		}

	/*case STLINK_MODE_DEBUG_JTAG:
	if h.version.jtag_api == STLINK_JTAG_API_V3 {
		return stlink_speed_v3(true, khz, query)
	} else {
		return stlink_speed_jtag(khz, query)
	}
	*/
	default:
		return khz, errors.New("Requested ST-Link mode not supported yet!")
	}
}

/** Issue an STLINK command via USB transfer, with retries on any wait status responses.

  Works for commands where the STLINK_DEBUG status is returned in the first
  byte of the response packet. For SWIM a SWIM_READSTATUS is requested instead.

  Returns an openocd result code.
*/
func (h *StLinkHandle) usb_cmd_allow_retry(buffer []byte, size int) error {
	var retries int = 0

	for true {
		if (h.st_mode != STLINK_MODE_DEBUG_SWIM) || retries > 0 {
			err := h.usb_xfer_noerrcheck(buffer, size)
			if err != nil {
				return err
			}
		}

		/* TODO: Implement DEBUG swim!
		if (h.st_mode == STLINK_MODE_DEBUG_SWIM) {
			err = h.stlink_swim_status(handle);
			if err != nil {
				return err
			}
		}*/

		err_code := h.usb_error_check()

		if err_code == ERROR_WAIT && retries < MAX_WAIT_RETRIES {
			var delay_us time.Duration = (1 << retries) * 1000

			retries++
			log.Debugf("stlink_cmd_allow_retry ERROR_WAIT, retry %d, delaying %u microseconds", retries, delay_us)
			time.Sleep(delay_us * 1000)

			continue
		}

		if err_code == ERROR_FAIL {
			return errors.New("Got error during usb check")
		} else {
			return nil
		}
	}

	return errors.New("Invalid allow cmd retry state")
}

func (h *StLinkHandle) usb_assert_srst(srst byte) error {

	/* TODO:
		* Implement SWIM debugger
	     *
		if h.st_mode == STLINK_MODE_DEBUG_SWIM {
			return stlink_swim_assert_reset(handle, srst);
		}
	*/

	if h.version.stlink == 1 {
		return errors.New("Could not find rsrt command on target")
	}

	h.usb_init_buffer(h.rx_ep, 2)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_DRIVE_NRST
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = srst
	h.cmdidx++

	return h.usb_cmd_allow_retry(h.databuf, 2)
}
