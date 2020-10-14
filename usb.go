// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"context"
	"errors"
	"time"

	"github.com/google/gousb"
)

var (
	libUsbCtx *gousb.Context = nil
)

func InitUsb() error {
	if libUsbCtx == nil {

		libUsbCtx = gousb.NewContext()
		libUsbCtx.Debug(3)

		if libUsbCtx != nil {
			return nil
		} else {
			return errors.New("could not initialize libusb context")
		}
	} else {
		logger.Warn("libusb context already initialized")
		return nil
	}
}

func CloseUSB() {
	if libUsbCtx != nil {
		libUsbCtx.Close()
	} else {
		logger.Warn("tried to close non initialized libusb context")
	}
}

func usbFindDevices(vids []gousb.ID, pids []gousb.ID) ([]*gousb.Device, error) {
	devices, err := libUsbCtx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		if idExists(vids, desc.Vendor) == true && idExists(pids, desc.Product) == true {
			logger.Debugf("inspecting usb device [%04x:%04x] on bus %03d:%03d...", uint16(desc.Vendor), uint16(desc.Product), desc.Bus, desc.Address)

			return true
		} else {
			return false
		}
	})

	// Error of OpenDevices is ignored cause of lack
	// of information on which specific device the error
	// occurred. So as long we got a valid device handle
	// returned there is no actual error

	if len(devices) > 0 {
		return devices, nil
	} else {
		return nil, err
	}
}

func usbRawWrite(endpoint *gousb.OutEndpoint, buffer []byte) (int, error) {

	opCtx := context.Background()

	var done func()
	opCtx, done = context.WithTimeout(opCtx, time.Millisecond*10000)
	defer done()

	bytesWritten, err := endpoint.WriteContext(opCtx, buffer)

	if err != nil {
		return -1, err
	} else {
		logger.Tracef("%d Bytes -> EP-%d", bytesWritten, endpoint.Desc.Number)
		return bytesWritten, nil
	}

}

func usbRawRead(endpoint *gousb.InEndpoint, buffer []byte) (int, error) {
	opCtx := context.Background()

	var done func()
	opCtx, done = context.WithTimeout(opCtx, time.Millisecond*50)
	defer done()

	bytesRead, err := endpoint.ReadContext(opCtx, buffer)

	if err != nil {
		return -1, err
	} else {
		logger.Tracef("EP-%d -> %d Bytes", endpoint.Desc.Number, bytesRead)
		return bytesRead, nil
	}
}

func (h *StLink) maxBlockSize(tarAutoIncrBlock uint32, address uint32) uint32 {
	var maxTarBlock = tarAutoIncrBlock - ((tarAutoIncrBlock - 1) & address)

	if maxTarBlock == 0 {
		maxTarBlock = 4
	}

	return maxTarBlock
}

func (h *StLink) usbBlock() uint32 {
	if h.version.flags.Get(flagHasRw8Bytes512) {
		return v3MaxReadWrite8
	} else {
		return maxReadWrite8
	}
}
