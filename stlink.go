// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.
package gostlink

import (
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

const HLA_MAX_USB_IDS = 8

var STLINK_SUPPORTED_VIDS = []uint16{0x0483} // STLINK Vendor ID
var STLINK_SUPPORTED_PIDS = []uint16{0x3744, 0x3748, 0x374b, 0x374d, 0x374e, 0x374f, 0x3752, 0x3753}

const STLINK_DEVICE_DESCRIPTION = "ST-Link"
const STLINK_DEVICE_SERIAL = ""
const STLINK_DEVICE_TRANSPORT = STLINK_MODE_DEBUG_SWD

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

type Stlink_usb_version struct {
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
type trace struct {
	/** whether SWO tracing is enabled or not */
	enabled bool
	/** trace module source clock */
	source_hz uint32
}

/** */
type stlink_usb_handle struct {
	/** */
	libusb_device *gousb.Device
	/** */
	libusb_config *gousb.Config
	/** */
	libusb_interface *gousb.Interface
	/** */
	rx_ep uint8
	/** */
	tx_ep uint8
	/** */
	trace_ep uint8
	/** */
	cmdbuf []uint8
	/** */
	cmdidx uint8
	/** */
	direction uint8
	/** */
	databuf []uint8
	/** */
	max_mem_packet uint32
	/** */
	st_mode StLinkMode
	/** */
	version Stlink_usb_version

	/** */
	vid uint16
	/** */
	pid uint16
	/** reconnect is needed next time we try to query the
	 * status */
	reconnect_pending bool
}

func OpenStLink(mode StLinkMode) (*stlink_usb_handle, error) {
	log.Debug("stlink_usb_open")

	var err error

	handle := &stlink_usb_handle{}
	handle.st_mode = mode

	usb_init()

	handle.libusb_device, err = usb_open_device(STLINK_SUPPORTED_VIDS, STLINK_SUPPORTED_PIDS, "")

	if err != nil {
		log.Fatal("Could not find any st link connected to computer.")
	}

	handle.libusb_config, err = handle.libusb_device.Config(1) // request for configuration #0

	if err != nil {
		log.Fatal("Could not request configuration #0 for st-link debugger.", err)
	}

	handle.libusb_interface, err = handle.libusb_config.Interface(0, 0)

	if err != nil {
		log.Fatal("Could not claim interface 0,0 for st-link debugger.")
	}

	handle.rx_ep = STLINK_RX_EP

	switch handle.libusb_device.Desc.Product {
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
		log.Infof("Could not determine pid of debugger %04x", handle.libusb_device.Desc.Product)
		handle.version.stlink = 2
		handle.tx_ep = STLINK_TX_EP
		handle.trace_ep = STLINK_TRACE_EP
	}

	log.Debugf("V: %d, TXEP: %02x RXEP: %02x, TEP: %02x", handle.version.stlink, handle.tx_ep, handle.rx_ep, handle.trace_ep)

	return handle, nil
}

func GetUsbVersion(h *stlink_usb_handle, usb_ver *Stlink_usb_version) bool {
	/*var res int
	var flags uint32
	*/
	var v, x, y uint8 = 0, 0, 0
	/*	var v_str [5*(1+3) + 1]uint8 VvJjMmBbSs

		var p *uint8
	*/
	stlink_usb_init_buffer(h, h.rx_ep, 6)

	h.cmdbuf[h.cmdidx] = STLINK_GET_VERSION
	h.cmdidx++

	stlink_usb_xfer_noerrcheck(h, h.databuf, 6)

	version := be_to_h_u16(h.databuf)

	v = uint8((version >> 12) & 0x0f)
	x = uint8((version >> 6) & 0x3f)
	y = uint8(version & 0x3f)

	h.vid = le_to_h_u16(h.databuf[2:])
	h.pid = le_to_h_u16(h.databuf[4:])

	log.Debugf("v: %d, x: %d, y: %d, VID: %04x, PID: %04x", v, x, y, h.vid, h.pid)

	return true
}

func Close(h *stlink_usb_handle) {
	h.libusb_device.Close()
	usb_close()
}

func stlink_usb_init_buffer(h *stlink_usb_handle, direction uint8, size uint32) {
	h.direction = direction
	h.cmdidx = 0

	if len(h.cmdbuf) == 0 {
		h.cmdbuf = make([]uint8, STLINK_SG_SIZE)
	}
	if len(h.databuf) == 0 {
		h.databuf = make([]uint8, STLINK_DATA_SIZE)
	}

	memset(h.cmdbuf, STLINK_SG_SIZE, 0)
	memset(h.databuf, STLINK_DATA_SIZE, 0)

	if h.version.stlink == 1 {
		stlink_usb_xfer_v1_create_cmd(h, direction, size)
	}
}

func stlink_usb_xfer_v1_create_cmd(h *stlink_usb_handle, direction uint8, size uint32) {
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

func stlink_usb_xfer_noerrcheck(h *stlink_usb_handle, buffer []byte, size int) {
	var _, cmdsize int = 0, STLINK_CMD_SIZE_V2

	if h.version.stlink == 1 {
		cmdsize = STLINK_SG_SIZE
		h.cmdbuf[14] = h.cmdidx - 15
	}

	stlink_usb_xfer_rw(h, cmdsize, buffer, size)
	/*

		if (h->version.stlink == 1) {
			if (stlink_usb_xfer_v1_get_status(handle) != ERROR_OK) {
				// check csw status
				if (h->cmdbuf[12] == 1) {
					LOG_DEBUG("get sense");
					if (stlink_usb_xfer_v1_get_sense(handle) != ERROR_OK)
						return ERROR_FAIL;
				}
				return ERROR_FAIL;
			}
		}
	*/
}

func stlink_usb_xfer_rw(h *stlink_usb_handle, cmdsize int, buffer []byte, size int) {
	// write command buffer to tx_ep
	outP, err := h.libusb_interface.OutEndpoint(int(h.tx_ep))

	if err != nil {
		log.Fatal("Could not open OEndpoint.", err)
	}

	usb_write(outP, h.cmdbuf[:cmdsize])

	if h.direction == h.tx_ep && size > 0 {
		usb_write(outP, buffer[:size])
	} else if h.direction == h.rx_ep && size > 0 {
		inP, err := h.libusb_interface.InEndpoint(int(h.rx_ep))

		if err != nil {
			log.Fatal("Could not get input endpoint of device")
		}

		usb_read(inP, buffer)
	}
}
