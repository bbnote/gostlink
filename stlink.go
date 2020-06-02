// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.
package gostlink

import (
	"fmt"

	"github.com/google/gousb"
	"github.com/google/gousb/usbid"
	log "github.com/sirupsen/logrus"
)

const (
	STLINK_SG_SIZE     = 31
	STLINK_DATA_SIZE   = 4096
	STLINK_CMD_SIZE_V2 = 16
	STLINK_CMD_SIZE_V1 = 10
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

type stlink_usb_version struct {
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
	//trans* libusb_transfer;   TODO: cover this api by golang usb library
	/** */
	rx_ep uint8
	/** */
	tx_ep uint8
	/** */
	trace_ep uint8
	/** */
	cmdbuf [STLINK_SG_SIZE]uint8
	/** */
	cmdidx uint8
	/** */
	direction uint8
	/** */
	databuf [STLINK_DATA_SIZE]uint8
	/** */
	max_mem_packet uint32
	/** */
	st_mode StLinkMode
	/** */
	version stlink_usb_version
	/** */
	vid uint16
	/** */
	pid uint16
	/** reconnect is needed next time we try to query the
	 * status */
	reconnect_pending bool
}

func OpenStLink(mode StLinkMode) (*stlink_usb_handle, error) {
	//var retry_count int = 1

	log.Debug("stlink_usb_open")

	handle := &stlink_usb_handle{}
	handle.st_mode = mode

	usb_init()

	device, _ := usb_open_device(STLINK_SUPPORTED_VIDS, STLINK_SUPPORTED_PIDS, "")

	// Test Implementation
	for _, cfg := range device.Desc.Configs {
		// This loop just uses more of the built-in and usbid pretty printing to list
		// the USB devices.
		fmt.Printf("  %s:\n", cfg)
		for _, intf := range cfg.Interfaces {
			fmt.Printf("    --------------\n")
			for _, ifSetting := range intf.AltSettings {
				fmt.Printf("    %s\n", ifSetting)
				fmt.Printf("      %s\n", usbid.Classify(ifSetting))
				for _, end := range ifSetting.Endpoints {
					fmt.Printf("      %s\n", end)
				}
			}
		}
		fmt.Printf("    --------------\n")
	}

	device.Close()

	usb_close()

	return handle, nil
}
