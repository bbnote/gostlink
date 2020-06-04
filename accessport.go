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

var opened_ap = bitmap.New(DP_APSEL_MAX + 1)

func (h *StLinkHandle) usb_open_ap(apsel uint16) error {

	/* nothing to do on old versions */
	if (h.version.flags & STLINK_F_HAS_AP_INIT) == 0 {
		return nil
	}

	if apsel > DP_APSEL_MAX {
		return errors.New("Apsel > DP_APSEL_MAX")
	}

	if opened_ap.Get(int(apsel)) {
		return nil
	}

	err := h.usb_init_access_port(byte(apsel))

	if err != nil {
		return err
	}

	log.Debugf("AP %d enabled", apsel)
	opened_ap.Set(int(apsel), true)
	return nil
}

func (h *StLinkHandle) usb_init_access_port(ap_num byte) error {
	if (h.version.flags & STLINK_F_HAS_AP_INIT) == 0 {
		return errors.New("Could not find access port command")
	}

	log.Debugf("init ap_num = %d", ap_num)

	h.usb_init_buffer(h.rx_ep, 16)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_INIT_AP
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = ap_num
	h.cmdidx++

	retval := h.usb_xfer_errcheck(h.databuf, 2)

	if retval != ERROR_OK {
		return errors.New("Could not init accessport on device")
	} else {
		return nil
	}
}
