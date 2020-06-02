// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"errors"

	log "github.com/sirupsen/logrus"

	"github.com/google/gousb"
)

var usb_ctx *gousb.Context = nil

func usb_init() {
	usb_ctx = gousb.NewContext()
	usb_ctx.Debug(2)

	if usb_ctx != nil {
		log.Debug("Initialized libsusb...")
	} else {
		log.Panic("Could not initialize libusb")
	}
}

func usb_open_device(vids []uint16, pids []uint16, serial_no string) (*gousb.Device, error) {
	var found_stlinks int = 0

	devices, err := usb_ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		if itemExists(vids, uint16(desc.Vendor)) == true {
			log.Infof("Found ST-Link on bus %d with address %d", desc.Bus, desc.Address)
			found_stlinks++

			return true
		} else {
			return false
		}

	})

	log.Infof("Found %d stlinks (%d)...", len(devices), found_stlinks)

	if found_stlinks == 1 {
		return devices[0], nil
	} else {
		return nil, errors.New("gostlink supports just one ST-Link at the moment")
	}

	return nil, err
}

func usb_close() {
	usb_ctx.Close()
}
