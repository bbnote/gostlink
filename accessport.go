// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"errors"

	"github.com/boljen/go-bitmap"
)

var (
	openedAp = bitmap.New(debugAccessPortSelectionMaximum + 1)
)

func (h *StLink) usbOpenAccessPort(apsel uint16) error {

	/* nothing to do on old versions */
	if !h.version.flags.Get(flagHasApInit) {
		return nil
	}

	if apsel > debugAccessPortSelectionMaximum {
		return errors.New("apsel > DP_APSEL_MAX")
	}

	if openedAp.Get(int(apsel)) {
		return nil
	}

	err := h.usbInitAccessPort(byte(apsel))

	if err != nil {
		return err
	}

	logger.Debugf("Access port %d enabled", apsel)
	openedAp.Set(int(apsel), true)
	return nil
}

func (h *StLink) usbInitAccessPort(apNum byte) error {
	if !h.version.flags.Get(flagHasApInit) {
		return errors.New("could not find access port command")
	}

	logger.Debugf("initialized access port # %d", apNum)

	ctx := h.initTransfer(transferRxEndpoint)

	ctx.cmdBuffer.WriteByte(cmdDebug)
	ctx.cmdBuffer.WriteByte(debugApiV2InitAccessPort)
	ctx.cmdBuffer.WriteByte(apNum)

	retVal := h.usbTransferErrCheck(ctx, 2)

	if retVal != nil {
		return errors.New("could not init access port on device")
	} else {
		return nil
	}
}
