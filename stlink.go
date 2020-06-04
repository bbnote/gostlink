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
	"math"
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

func NewStLink(vid gousb.ID, pid gousb.ID, serial_no string, mode StLinkMode) (*StLinkHandle, error) {
	var err error
	var devices []*gousb.Device

	handle := &StLinkHandle{}
	handle.st_mode = mode

	if vid == STLINK_ALL_VIDS && pid == STLINK_ALL_PIDS {
		devices, err = usb_find_devices(stlink_supported_vids, stlink_supported_pids)

	} else if vid == STLINK_ALL_VIDS && pid != STLINK_ALL_PIDS {
		devices, err = usb_find_devices(stlink_supported_vids, []gousb.ID{pid})

	} else if vid != STLINK_ALL_VIDS && pid == STLINK_ALL_PIDS {
		devices, err = usb_find_devices([]gousb.ID{vid}, stlink_supported_pids)

	} else {
		devices, err = usb_find_devices([]gousb.ID{vid}, []gousb.ID{pid})
	}

	if err != nil {
		return nil, err
	}

	if len(devices) > 0 {

		if serial_no == "" && len(devices) > 1 {
			return nil, errors.New("Could not idendify exact stlink by given paramters. (Perhaps a serial no is missing?)")
		} else if len(devices) == 1 {
			handle.usb_device = devices[0]
		} else {
			for _, dev := range devices {
				dev_serial_no, _ := dev.SerialNumber()

				log.Debugf("Compare serial no %s with number %s", dev_serial_no, serial_no)

				if dev_serial_no == serial_no {
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

	// initialize data buffers for tx and rx
	handle.cmdbuf = make([]byte, STLINK_SG_SIZE)
	handle.databuf = make([]byte, STLINK_DATA_SIZE)

	handle.stlink_version()

	return handle, nil
}

func (h *StLinkHandle) stlink_version() error {
	var v, x, y, jtag, swim, msd, bridge byte = 0, 0, 0, 0, 0, 0, 0

	h.stlink_init_buffer(h.rx_ep, 6)

	h.cmdbuf[h.cmdidx] = STLINK_GET_VERSION
	h.cmdidx++

	err := h.stlink_xfer_noerrcheck(h.databuf, 6)

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
		h.stlink_init_buffer(h.rx_ep, 16)

		h.cmdbuf[h.cmdidx] = STLINK_APIV3_GET_VERSION_EX
		h.cmdidx++

		err := h.stlink_xfer_noerrcheck(h.databuf, 12)

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

func (h *StLinkHandle) Close() {
	if h.usb_device != nil {
		log.Debugf("Close ST-Link device [%04x:%04x]", uint16(h.vid), uint16(h.pid))

		h.usb_interface.Close()
		h.usb_config.Close()
		h.usb_device.Close()
	}
}

func (h *StLinkHandle) stlink_init_buffer(direction byte, size uint32) {
	h.direction = direction
	h.cmdidx = 0

	memset(h.cmdbuf, STLINK_SG_SIZE, 0)
	memset(h.databuf, STLINK_DATA_SIZE, 0)

	if h.version.stlink == 1 {
		h.stlink_xfer_v1_create_cmd(direction, size)
	}
}

func (h *StLinkHandle) stlink_xfer_v1_create_cmd(direction uint8, size uint32) {
	h.cmdbuf[0] = 'U'
	h.cmdbuf[1] = 'S'
	h.cmdbuf[2] = 'B'
	h.cmdbuf[3] = 'C'
	h.cmdidx += 4

	buf_set_u32(h.cmdbuf[:h.cmdidx], 0, 32, 0)
	h.cmdidx += 4

	buf_set_u32(h.cmdbuf[:h.cmdidx], 0, 32, size)
	h.cmdidx += 4

	/* cbw flags */
	if direction == h.rx_ep {
		h.cmdbuf[h.cmdidx] = ENDPOINT_IN
	} else {
		h.cmdbuf[h.cmdidx] = ENDPOINT_OUT
	}
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = 0 /* lun */
	h.cmdidx++

	/* cdb clength (is filled in at xfer) */
	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++
}

func (h *StLinkHandle) stlink_xfer_noerrcheck(buffer []byte, size int) error {
	var cmdsize int = STLINK_CMD_SIZE_V2

	if h.version.stlink == 1 {
		cmdsize = STLINK_SG_SIZE
		h.cmdbuf[14] = h.cmdidx - 15
	}

	err := h.stlink_xfer_rw(cmdsize, buffer, size)

	if err != nil {
		return err
	}

	if h.version.stlink == 1 {
		err := h.stlink_xfer_v1_get_status()

		if err == nil {
			if h.cmdbuf[12] == 1 {
				log.Debug("Check sense")

				err = h.stlink_xfer_v1_get_sense()
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (h *StLinkHandle) stlink_xfer_errcheck(buffer []byte, size int) int {

	err := h.stlink_xfer_noerrcheck(buffer, size)

	if err != nil {
		log.Error(err)
		return ERROR_FAIL
	}

	return h.stlink_usb_error_check()
}

func (h *StLinkHandle) stlink_xfer_rw(cmdsize int, buffer []byte, size int) error {
	// write command buffer to tx_ep
	outP, err := h.usb_interface.OutEndpoint(int(h.tx_ep))

	if err != nil {
		return errors.New("Could not open out endpoint")
	}

	_, err = usb_write(outP, h.cmdbuf[:cmdsize])

	if err != nil {
		return err
	}

	if h.direction == h.tx_ep && size > 0 {
		_, err = usb_write(outP, buffer[:size])

		if err != nil {
			return err
		}

	} else if h.direction == h.rx_ep && size > 0 {

		inP, err := h.usb_interface.InEndpoint(int(h.rx_ep))

		if err != nil {
			return errors.New("Could not get in endpoint")
		}

		_, err = usb_read(inP, buffer)

		if err != nil {
			return err
		}
	}

	return nil
}

/**
  Converts an STLINK status code held in the first byte of a response
  to an openocd error, logs any error/wait status as debug output.
*/
func (h *StLinkHandle) stlink_usb_error_check() int {

	if h.st_mode == STLINK_MODE_DEBUG_SWIM {
		switch h.databuf[0] {
		case STLINK_SWIM_ERR_OK:
			return ERROR_OK

		case STLINK_SWIM_BUSY:
			log.Debug("SWIM Busy")
			return ERROR_WAIT

		default:
			log.Debugf("unknown/unexpected STLINK status code 0x%x", h.databuf[0])
			return ERROR_FAIL
		}
	}

	/* TODO: no error checking yet on api V1 */
	if h.version.jtag_api == STLINK_JTAG_API_V1 {
		h.databuf[0] = STLINK_DEBUG_ERR_OK
	}

	switch h.databuf[0] {
	case STLINK_DEBUG_ERR_OK:
		return ERROR_OK

	case STLINK_DEBUG_ERR_FAULT:
		log.Debugf("SWD fault response (0x%x)", STLINK_DEBUG_ERR_FAULT)
		return ERROR_FAIL
	case STLINK_SWD_AP_WAIT:
		log.Debugf("wait status SWD_AP_WAIT (0x%x)", STLINK_SWD_AP_WAIT)
		return ERROR_WAIT
	case STLINK_SWD_DP_WAIT:
		log.Debugf("wait status SWD_DP_WAIT (0x%x)", STLINK_SWD_DP_WAIT)
		return ERROR_WAIT
	case STLINK_JTAG_GET_IDCODE_ERROR:
		log.Debug("STLINK_JTAG_GET_IDCODE_ERROR")
		return ERROR_FAIL
	case STLINK_JTAG_WRITE_ERROR:
		log.Debug("Write error")
		return ERROR_FAIL
	case STLINK_JTAG_WRITE_VERIF_ERROR:
		log.Debug("Write verify error, ignoring")
		return ERROR_OK
	case STLINK_SWD_AP_FAULT:
		/* git://git.ac6.fr/openocd commit 657e3e885b9ee10
		 * returns ERROR_OK with the comment:
		 * Change in error status when reading outside RAM.
		 * This fix allows CDT plugin to visualize memory.
		 */
		log.Debug("STLINK_SWD_AP_FAULT")
		return ERROR_FAIL
	case STLINK_SWD_AP_ERROR:
		log.Debug("STLINK_SWD_AP_ERROR")
		return ERROR_FAIL
	case STLINK_SWD_AP_PARITY_ERROR:
		log.Debug("STLINK_SWD_AP_PARITY_ERROR")
		return ERROR_FAIL
	case STLINK_SWD_DP_FAULT:
		log.Debug("STLINK_SWD_DP_FAULT")
		return ERROR_FAIL
	case STLINK_SWD_DP_ERROR:
		log.Debug("STLINK_SWD_DP_ERROR")
		return ERROR_FAIL
	case STLINK_SWD_DP_PARITY_ERROR:
		log.Debug("STLINK_SWD_DP_PARITY_ERROR")
		return ERROR_FAIL
	case STLINK_SWD_AP_WDATA_ERROR:
		log.Debug("STLINK_SWD_AP_WDATA_ERROR")
		return ERROR_FAIL
	case STLINK_SWD_AP_STICKY_ERROR:
		log.Debug("STLINK_SWD_AP_STICKY_ERROR")
		return ERROR_FAIL
	case STLINK_SWD_AP_STICKYORUN_ERROR:
		log.Debug("STLINK_SWD_AP_STICKYORUN_ERROR")
		return ERROR_FAIL
	case STLINK_BAD_AP_ERROR:
		log.Debug("STLINK_BAD_AP_ERROR")
		return ERROR_FAIL
	default:
		log.Debugf("unknown/unexpected STLINK status code 0x%x", h.databuf[0])
		return ERROR_FAIL
	}
}

func (h *StLinkHandle) stlink_xfer_v1_get_status() error {
	memset(h.cmdbuf, STLINK_SG_SIZE, 0)

	in_endpoint, err := h.usb_interface.InEndpoint(int(h.rx_ep))

	if err != nil {
		return err
	}

	var b_read int = 0

	b_read, err = usb_read(in_endpoint, h.cmdbuf)

	if err != nil || b_read != 13 {
		return errors.New("ST-Link V1 status read error")
	}

	t1 := buf_get_u32(h.cmdbuf, 0, 32)

	/* check for USBS */
	if t1 != 0x53425355 {
		return errors.New("No USBS")
	}

	/*
	 * CSW status:
	 * 0 success
	 * 1 command failure
	 * 2 phase error
	 */
	if h.cmdbuf[12] != 0 {
		log.Errorf("Got CSW status: %d", h.cmdbuf[12])
		return errors.New("GOT CSW status error")
	}

	return nil
}

func (h *StLinkHandle) stlink_xfer_v1_get_sense() error {

	h.stlink_init_buffer(h.rx_ep, 16)

	h.cmdbuf[h.cmdidx] = REQUEST_SENSE
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = REQUEST_SENSE_LENGTH

	err := h.stlink_xfer_rw(REQUEST_SENSE_LENGTH, h.databuf, 16)

	if err != nil {
		return err
	} else {
		err := h.stlink_xfer_v1_get_status()
		return err
	}
}

func (h *StLinkHandle) stlink_usb_current_mode() (byte, error) {

	h.stlink_init_buffer(h.rx_ep, 2)

	h.cmdbuf[h.cmdidx] = STLINK_GET_CURRENT_MODE
	h.cmdidx++

	err := h.stlink_xfer_noerrcheck(h.databuf, 2)

	if err != nil {
		return 0, err
	} else {
		return h.databuf[0], nil
	}
}

func (h *StLinkHandle) stlink_usb_init_mode(connect_under_reset bool, initial_interface_speed int) error {

	mode, err := h.stlink_usb_current_mode()

	if err != nil {
		log.Error("Could not get usb mode")
		return err
	}

	log.Debugf("Got usb mode: %d", mode)

	var stlink_mode StLinkMode

	switch mode {
	case STLINK_DEV_DFU_MODE:
		stlink_mode = STLINK_MODE_DFU

	case STLINK_DEV_DEBUG_MODE:
		stlink_mode = STLINK_MODE_DEBUG_SWD

	case STLINK_DEV_SWIM_MODE:
		stlink_mode = STLINK_MODE_DEBUG_SWIM

	case STLINK_DEV_BOOTLOADER_MODE, STLINK_DEV_MASS_MODE:
		stlink_mode = STLINK_MODE_UNKNOWN
	default:
		stlink_mode = STLINK_MODE_UNKNOWN
	}

	if stlink_mode != STLINK_MODE_UNKNOWN {
		h.stlink_usb_leave_mode(stlink_mode)
	}

	mode, err = h.stlink_usb_current_mode()

	if err != nil {
		log.Error("Could not get usb mode")
		return err
	}

	/* we check the target voltage here as an aid to debugging connection problems.
	 * the stlink requires the target Vdd to be connected for reliable debugging.
	 * this cmd is supported in all modes except DFU
	 */
	if mode != STLINK_DEV_DFU_MODE {
		/* check target voltage (if supported) */
		voltage, err := h.stlink_check_voltage()

		if err != nil {
			log.Error(err)
			// attempt to continue as it is not a catastrophic failure
		} else {
			if voltage < 1.5 {
				log.Error("target voltage may be too low for reliable debugging")
			}
		}
	}

	log.Debugf("MODE: 0x%02X", mode)

	stlink_mode = h.st_mode

	if stlink_mode == STLINK_MODE_UNKNOWN {
		return errors.New("Selected mode (transport) not supported")
	}

	if stlink_mode == STLINK_MODE_DEBUG_JTAG {
		if (h.version.flags & STLINK_F_HAS_JTAG_SET_FREQ) != 0 {
			stlink_dump_speed_map(stlink_khz_to_speed_map_jtag[:])
			h.stlink_speed(initial_interface_speed, false)
		}
	} else if stlink_mode == STLINK_MODE_DEBUG_SWD {
		if (h.version.flags & STLINK_F_HAS_JTAG_SET_FREQ) != 0 {
			stlink_dump_speed_map(stlink_khz_to_speed_map_swd[:])
			h.stlink_speed(initial_interface_speed, false)
		}
	}

	if h.version.jtag_api == STLINK_JTAG_API_V3 {
		var smap = make([]speed_map, STLINK_V3_MAX_FREQ_NB)

		h.stlink_get_com_freq(stlink_mode == STLINK_MODE_DEBUG_JTAG, &smap)
		stlink_dump_speed_map(smap)
		h.stlink_speed(initial_interface_speed, false)
	}

	// preliminary SRST assert:
	//  We want SRST is asserted before activating debug signals (mode_enter).
	//  As the required mode has not been set, the adapter may not know what pin to use.
	//  Tested firmware STLINK v2 JTAG v29 API v2 SWIM v0 uses T_NRST pin by default
	//  Tested firmware STLINK v2 JTAG v27 API v2 SWIM v6 uses T_NRST pin by default
	//  after power on, SWIM_RST stays unchanged
	if connect_under_reset && stlink_mode != STLINK_MODE_DEBUG_SWIM {
		h.stlink_usb_assert_srst(0)
		// do not check the return status here, we will
		//   proceed and enter the desired mode below
		//   and try asserting srst again.
	}

	err = h.stlink_usb_mode_enter(stlink_mode)

	if err != nil {
		return err
	}

	if connect_under_reset {
		err = h.stlink_usb_assert_srst(0)
		if err != nil {
			return err
		}
	}

	mode, err = h.stlink_usb_current_mode()

	if err != nil {
		return err
	}

	log.Debugf("Mode: 0x%02x", mode)

	return nil
}

func (h *StLinkHandle) stlink_usb_leave_mode(mode StLinkMode) error {
	h.stlink_init_buffer(h.rx_ep, 0)

	switch mode {
	case STLINK_MODE_DEBUG_JTAG, STLINK_MODE_DEBUG_SWD:
		h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
		h.cmdidx++
		h.cmdbuf[h.cmdidx] = STLINK_DEBUG_EXIT
		h.cmdidx++

	case STLINK_MODE_DEBUG_SWIM:
		h.cmdbuf[h.cmdidx] = STLINK_SWIM_COMMAND
		h.cmdidx++
		h.cmdbuf[h.cmdidx] = STLINK_SWIM_EXIT
		h.cmdidx++

	case STLINK_MODE_DFU:
		h.cmdbuf[h.cmdidx] = STLINK_DFU_COMMAND
		h.cmdidx++
		h.cmdbuf[h.cmdidx] = STLINK_DFU_EXIT
		h.cmdidx++

	case STLINK_MODE_MASS:
		return errors.New("Unknown stlink mode")
	default:
		return errors.New("Unknown stlink mode")
	}

	err := h.stlink_xfer_noerrcheck(h.databuf, 0)

	return err
}

/** Issue an STLINK command via USB transfer, with retries on any wait status responses.

  Works for commands where the STLINK_DEBUG status is returned in the first
  byte of the response packet. For SWIM a SWIM_READSTATUS is requested instead.

  Returns an openocd result code.
*/
func (h *StLinkHandle) stlink_cmd_allow_retry(buffer []byte, size int) error {
	var retries int = 0

	for true {
		if (h.st_mode != STLINK_MODE_DEBUG_SWIM) || retries > 0 {
			err := h.stlink_xfer_noerrcheck(buffer, size)
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

		err_code := h.stlink_usb_error_check()

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

func (h *StLinkHandle) stlink_usb_assert_srst(srst byte) error {

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

	h.stlink_init_buffer(h.rx_ep, 2)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_DRIVE_NRST
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = srst
	h.cmdidx++

	return h.stlink_cmd_allow_retry(h.databuf, 2)
}

func (h *StLinkHandle) stlink_check_voltage() (float32, error) {
	var adc_results [2]uint32

	/* no error message, simply quit with error */
	if (h.version.flags & STLINK_F_HAS_TARGET_VOLT) == 0 {
		return -1.0, errors.New("Device does not support voltage measurement")
	}

	h.stlink_init_buffer(h.rx_ep, 8)

	h.cmdbuf[h.cmdidx] = STLINK_GET_TARGET_VOLTAGE
	h.cmdidx++

	err := h.stlink_xfer_noerrcheck(h.databuf, 8)

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

/** */
func (h *StLinkHandle) stlink_usb_mode_enter(st_mode StLinkMode) error {
	var rx_size uint32 = 0
	/* on api V2 we are able the read the latest command
	 * status
	 * TODO: we need the test on api V1 too
	 */
	if h.version.jtag_api != STLINK_JTAG_API_V1 {
		rx_size = 2
	}

	h.stlink_init_buffer(h.rx_ep, rx_size)

	switch st_mode {
	case STLINK_MODE_DEBUG_JTAG:
		h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
		h.cmdidx++

		if h.version.jtag_api == STLINK_JTAG_API_V1 {
			h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV1_ENTER
		} else {
			h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_ENTER
		}
		h.cmdidx++

		h.cmdbuf[h.cmdidx] = STLINK_DEBUG_ENTER_JTAG_NO_RESET
		h.cmdidx++

	case STLINK_MODE_DEBUG_SWD:
		h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
		h.cmdidx++

		if h.version.jtag_api == STLINK_JTAG_API_V1 {
			h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV1_ENTER
		} else {
			h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_ENTER
		}
		h.cmdidx++

		h.cmdbuf[h.cmdidx] = STLINK_DEBUG_ENTER_SWD_NO_RESET
		h.cmdidx++

	case STLINK_MODE_DEBUG_SWIM:
		h.cmdbuf[h.cmdidx] = STLINK_SWIM_COMMAND
		h.cmdidx++
		h.cmdbuf[h.cmdidx] = STLINK_SWIM_ENTER
		h.cmdidx++

		/* swim enter does not return any response or status */
		return h.stlink_xfer_noerrcheck(h.databuf, 0)
	case STLINK_MODE_DFU:
	case STLINK_MODE_MASS:
	default:
		return errors.New("Cannot set usb mode from DFU or mass stlink configuration")
	}

	return h.stlink_cmd_allow_retry(h.databuf, int(rx_size))
}

func (h *StLinkHandle) stlink_speed(khz int, query bool) (int, error) {

	switch h.st_mode {
	/*case STLINK_MODE_DEBUG_SWIM:
	return stlink_speed_swim(khz, query)
	*/

	case STLINK_MODE_DEBUG_SWD:
		if h.version.jtag_api == STLINK_JTAG_API_V3 {
			return h.stlink_speed_v3(false, khz, query)
		} else {
			return h.stlink_speed_swd(khz, query)
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

func (h *StLinkHandle) stlink_speed_v3(is_jtag bool, khz int, query bool) (int, error) {

	var smap = make([]speed_map, STLINK_V3_MAX_FREQ_NB)

	h.stlink_get_com_freq(is_jtag, &smap)

	speed_index, err := stlink_match_speed_map(smap, khz, query)

	if err != nil {
		return khz, err
	}

	if !query {
		err := h.stlink_set_com_freq(is_jtag, smap[speed_index].speed)

		if err != nil {
			return khz, err
		}
	}

	return smap[speed_index].speed, nil
}

func (h *StLinkHandle) stlink_speed_swd(khz int, query bool) (int, error) {

	/* old firmware cannot change it */
	if (h.version.flags & STLINK_F_HAS_SWD_SET_FREQ) == 0 {
		return khz, errors.New("Cannot change speed on old firmware")
	}

	speed_index, err := stlink_match_speed_map(stlink_khz_to_speed_map_swd[:], khz, query)

	if err != nil {
		return khz, err
	}

	if !query {
		error := h.stlink_usb_set_swdclk(uint16(stlink_khz_to_speed_map_swd[speed_index].speed_divisor))

		if error != nil {
			return khz, errors.New("Unable to set adapter speed")
		}
	}

	return stlink_khz_to_speed_map_swd[speed_index].speed, nil
}

func (h *StLinkHandle) stlink_usb_set_swdclk(clk_divisor uint16) error {

	if (h.version.flags & STLINK_F_HAS_SWD_SET_FREQ) == 0 {
		errors.New("Cannot change speed on this firmware")
	}

	h.stlink_init_buffer(h.rx_ep, 2)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_SWD_SET_FREQ
	h.cmdidx++

	h_u16_to_le(h.cmdbuf[h.cmdidx:], int(clk_divisor))
	h.cmdidx += 2

	err := h.stlink_cmd_allow_retry(h.databuf, 2)

	return err
}

func (h *StLinkHandle) stlink_get_com_freq(is_jtag bool, smap *[]speed_map) error {

	if h.version.jtag_api != STLINK_JTAG_API_V3 {
		errors.New("Unknown command")
	}

	h.stlink_init_buffer(h.rx_ep, 16)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = STLINK_APIV3_GET_COM_FREQ
	h.cmdidx++

	if is_jtag {
		h.cmdbuf[h.cmdidx] = 1
	} else {
		h.cmdbuf[h.cmdidx] = 0
	}
	h.cmdidx++

	err := h.stlink_xfer_errcheck(h.databuf, 52)

	var size int = int(h.databuf[8])

	if size > STLINK_V3_MAX_FREQ_NB {
		size = STLINK_V3_MAX_FREQ_NB
	}

	for i := 0; i < size; i++ {
		(*smap)[i].speed = int(le_to_h_u32(h.databuf[12+4*i:]))
		(*smap)[i].speed_divisor = i
	}

	// set to zero all the next entries
	for i := size; i < STLINK_V3_MAX_FREQ_NB; i++ {
		(*smap)[i].speed = 0
	}

	if err == ERROR_OK {
		return nil
	} else {
		return errors.New("Got error check fail")
	}
}

func (h *StLinkHandle) stlink_set_com_freq(is_jtag bool, frequency int) error {

	if h.version.jtag_api != STLINK_JTAG_API_V3 {
		return errors.New("Unknown command")
	}

	h.stlink_init_buffer(h.rx_ep, 16)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = STLINK_APIV3_SET_COM_FREQ
	h.cmdidx++

	if is_jtag {
		h.cmdbuf[h.cmdidx] = 1
	} else {
		h.cmdbuf[h.cmdidx] = 0
	}
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++

	h_u32_to_le(h.cmdbuf[4:], frequency)

	err := h.stlink_xfer_errcheck(h.databuf, 8)

	if err == ERROR_OK {
		return nil
	} else {
		return errors.New("Got error check fail")
	}
}

func stlink_match_speed_map(smap []speed_map, khz int, query bool) (int, error) {
	var last_valid_speed int = -1
	var speed_index = -1
	var speed_diff = math.MaxInt32
	var match bool = false
	var counter int = 0

	for i, s := range smap {
		counter = i
		if s.speed == 0 {
			continue
		}

		last_valid_speed = i
		if khz == s.speed {
			speed_index = i
			break
		} else {
			var current_diff = khz - s.speed

			//get abs value for comparison
			if current_diff <= 0 {
				current_diff = -current_diff
			}

			if (current_diff < speed_diff) && khz >= s.speed {
				speed_diff = current_diff
				speed_index = i
			}
		}
	}

	if speed_index == -1 {
		// this will only be here if we cannot match the slow speed.
		// use the slowest speed we support.
		speed_index = last_valid_speed
		match = false
	} else if counter == len(smap) {
		match = false
	}

	if !match && query {
		return -1, errors.New(fmt.Sprintf("Unable to match requested speed %d kHz, using %d kHz",
			khz, smap[speed_index].speed))
	}

	return speed_index, nil
}

func stlink_dump_speed_map(smap []speed_map) {
	for i := range smap {
		if smap[i].speed > 0 {
			log.Debugf("%d kHz", smap[i].speed)
		}
	}
}
