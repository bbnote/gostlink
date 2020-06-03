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

	"github.com/google/gousb"
	log "github.com/sirupsen/logrus"
)

const (
	STLINK_SG_SIZE     = 31
	STLINK_DATA_SIZE   = 4096
	STLINK_CMD_SIZE_V2 = 16
	STLINK_CMD_SIZE_V1 = 10
	STLINK_SERIAL_LEN  = 24
)

const STLINK_ALL_VIDS = 0xFFFF
const STLINK_ALL_PIDS = 0xFFFF

var stlink_supported_vids = []gousb.ID{0x0483} // STLINK Vendor ID
var stlink_supported_pids = []gousb.ID{0x3744, 0x3748, 0x374b, 0x374d, 0x374e, 0x374f, 0x3752, 0x3753}

type StLinkMode uint8

const (
	STLINK_MODE_UNKNOWN    StLinkMode = 0
	STLINK_MODE_DFU                   = 1
	STLINK_MODE_MASS                  = 2
	STLINK_MODE_DEBUG_JTAG            = 3
	STLINK_MODE_DEBUG_SWD             = 4
	STLINK_MODE_DEBUG_SWIM            = 5
)

type StLinkApiVersion uint8

const (
	STLINK_JTAG_API_V1 StLinkApiVersion = 1
	STLINK_JTAG_API_V2                  = 2
	STLINK_JTAG_API_V3                  = 3
)

const (
	STLINK_V1_PID           = 0x3744
	STLINK_V2_PID           = 0x3748
	STLINK_V2_1_PID         = 0x374B
	STLINK_V2_1_NO_MSD_PID  = 0x3752
	STLINK_V3_USBLOADER_PID = 0x374D
	STLINK_V3E_PID          = 0x374E
	STLINK_V3S_PID          = 0x374F
	STLINK_V3_2VCP_PID      = 0x3753
)

const (
	ENDPOINT_IN  = 0x80
	ENDPOINT_OUT = 0x00

	STLINK_WRITE_TIMEOUT = 1000
	STLINK_READ_TIMEOUT  = 1000

	STLINK_RX_EP    = (1 | ENDPOINT_IN)
	STLINK_TX_EP    = (2 | ENDPOINT_OUT)
	STLINK_TRACE_EP = (3 | ENDPOINT_IN)

	STLINK_V2_1_TX_EP    = (1 | ENDPOINT_OUT)
	STLINK_V2_1_TRACE_EP = (2 | ENDPOINT_IN)
)

const (
	STLINK_GET_VERSION        = 0xF1
	STLINK_DEBUG_COMMAND      = 0xF2
	STLINK_DFU_COMMAND        = 0xF3
	STLINK_SWIM_COMMAND       = 0xF4
	STLINK_GET_CURRENT_MODE   = 0xF5
	STLINK_GET_TARGET_VOLTAGE = 0xF7
)

const (
	STLINK_DEBUG_PORT_ACCESS = 0xffff

	STLINK_TRACE_SIZE   = 4096
	STLINK_TRACE_MAX_HZ = 2000000

	STLINK_V3_MAX_FREQ_NB       = 10
	STLINK_APIV3_GET_VERSION_EX = 0xFB

	REQUEST_SENSE        = 0x03
	REQUEST_SENSE_LENGTH = 18
)

const (
	STLINK_F_HAS_TRACE            = 0x01
	STLINK_F_HAS_SWD_SET_FREQ     = 0x02
	STLINK_F_HAS_JTAG_SET_FREQ    = 0x04
	STLINK_F_HAS_MEM_16BIT        = 0x08
	STLINK_F_HAS_GETLASTRWSTATUS2 = 0x10
	STLINK_F_HAS_DAP_REG          = 0x20
	STLINK_F_QUIRK_JTAG_DP_READ   = 0x40
	STLINK_F_HAS_AP_INIT          = 0x80
	STLINK_F_HAS_DPBANKSEL        = 0x100
	STLINK_F_HAS_RW8_512BYTES     = 0x200
	STLINK_F_FIX_CLOSE_AP         = 0x400
)

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
