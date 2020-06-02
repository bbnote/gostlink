// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

// this code is mainly inspired and based on the openocd project source code
// for detailed information see

// https://sourceforge.net/p/openocd/code
package gostlink

import (
	"errors"

	"github.com/boljen/go-bitmap"
	log "github.com/sirupsen/logrus"
)

var openedAp = bitmap.New(debugAccessPortSelectionMaximum + 1)

func (h *StLinkHandle) usbOpenAp(apsel uint16) error {

	/* nothing to do on old versions */
	if (h.version.flags & flagHasApInit) == 0 {
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

	log.Debugf("AP %d enabled", apsel)
	openedAp.Set(int(apsel), true)
	return nil
}

func (h *StLinkHandle) usbInitAccessPort(apNum byte) error {
	if (h.version.flags & flagHasApInit) == 0 {
		return errors.New("could not find access port command")
	}

	log.Debugf("init ap_num = %d", apNum)

	h.usbInitBuffer(transferRxEndpoint, 16)

	h.cmdbuf[h.cmdidx] = cmdDebug
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = debugApiV2InitAccessPort
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = apNum
	h.cmdidx++

	retVal := h.usbTransferErrCheck(h.databuf, 2)

	if retVal != nil {
		return errors.New("could not init access port on device")
	} else {
		return nil
	}
}
