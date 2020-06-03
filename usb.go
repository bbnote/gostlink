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

func InitializeUSB() error {
	if usb_ctx == nil {
		usb_ctx = gousb.NewContext()
		usb_ctx.Debug(2)

		if usb_ctx != nil {
			log.Debug("Initialized libsusb...")
			return nil
		} else {
			return errors.New("Could not initialize libusb!")
		}
	} else {
		log.Warn("USB already initialized!")
		return nil
	}
}

func CloseUSB() {
	if usb_ctx != nil {
		usb_ctx.Close()
	} else {
		log.Warn("Could not close uninitialized usb context")
	}
}

func usb_find_devices(vids []gousb.ID, pids []gousb.ID) ([]*gousb.Device, error) {
	devices, err := usb_ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		if idExists(vids, desc.Vendor) == true {
			log.Infof("Found USB device [%04x:%04x] on bus %03d:%03d", uint16(desc.Vendor), uint16(desc.Product), desc.Bus, desc.Address)

			return true
		} else {
			return false
		}
	})

	if err == nil {
		log.Infof("Found %d matching devices based on vendor and product id list", len(devices))
		return devices, nil
	} else {
		log.Error("Got error during usb device scan", err)
		return nil, err
	}
}

func usb_write(endpoint *gousb.OutEndpoint, buffer []byte) (int, error) {
	b_written, err := endpoint.Write(buffer)

	if err != nil {
		return -1, err
	} else {
		log.Tracef("Wrote %d bytes to endpoint", b_written)
		return b_written, nil
	}
}

func usb_read(endpoint *gousb.InEndpoint, buffer []byte) (int, error) {
	b_read, err := endpoint.Read(buffer)

	if err != nil {
		return -1, err
	} else {
		log.Tracef("Read %d byte from in endpoint", b_read)
		return b_read, nil
	}
}
