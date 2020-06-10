// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

// this code is mainly inspired and based on the openocd project source code
// for detailed information see

// https://sourceforge.net/p/openocd/code

package gostlink

import (
	"errors"

	log "github.com/sirupsen/logrus"
)

/** */
func (h *StLinkHandle) usbModeEnter(stMode StLinkMode) error {
	var rxSize uint32 = 0
	/* on api V2 we are able the read the latest command
	 * status
	 * TODO: we need the test on api V1 too
	 */
	if h.version.jtag_api != STLINK_JTAG_API_V1 {
		rxSize = 2
	}

	h.usbInitBuffer(h.rx_ep, rxSize)

	switch stMode {
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
		return h.usbTransferNoErrCheck(h.databuf, 0)
	case STLINK_MODE_DFU:
	case STLINK_MODE_MASS:
	default:
		return errors.New("cannot set usb mode from DFU or mass stlink configuration")
	}

	return h.usbCmdAllowRetry(h.databuf, rxSize)
}

func (h *StLinkHandle) usbCurrentMode() (byte, error) {

	h.usbInitBuffer(h.rx_ep, 2)

	h.cmdbuf[h.cmdidx] = STLINK_GET_CURRENT_MODE
	h.cmdidx++

	err := h.usbTransferNoErrCheck(h.databuf, 2)

	if err != nil {
		return 0, err
	} else {
		return h.databuf[0], nil
	}
}

func (h *StLinkHandle) usbInitMode(connectUnderReset bool, initialInterfaceSpeed uint32) error {

	mode, err := h.usbCurrentMode()

	if err != nil {
		log.Error("Could not get usb mode")
		return err
	}

	log.Debugf("Got usb mode: %d", mode)

	var stLinkMode StLinkMode

	switch mode {
	case STLINK_DEV_DFU_MODE:
		stLinkMode = STLINK_MODE_DFU

	case STLINK_DEV_DEBUG_MODE:
		stLinkMode = STLINK_MODE_DEBUG_SWD

	case STLINK_DEV_SWIM_MODE:
		stLinkMode = STLINK_MODE_DEBUG_SWIM

	case STLINK_DEV_BOOTLOADER_MODE, STLINK_DEV_MASS_MODE:
		stLinkMode = STLINK_MODE_UNKNOWN

	default:
		stLinkMode = STLINK_MODE_UNKNOWN
	}

	if stLinkMode != STLINK_MODE_UNKNOWN {
		h.usbLeaveMode(stLinkMode)
	}

	mode, err = h.usbCurrentMode()

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
		voltage, err := h.GetTargetVoltage()

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

	stLinkMode = h.st_mode

	if stLinkMode == STLINK_MODE_UNKNOWN {
		return errors.New("Selected mode (transport) not supported")
	}

	if stLinkMode == STLINK_MODE_DEBUG_JTAG {
		if (h.version.flags & STLINK_F_HAS_JTAG_SET_FREQ) != 0 {
			dumpSpeedMap(JTAGkHzToSpeedMap[:])
			h.SetSpeed(initialInterfaceSpeed, false)
		}
	} else if stLinkMode == STLINK_MODE_DEBUG_SWD {
		if (h.version.flags & STLINK_F_HAS_JTAG_SET_FREQ) != 0 {
			dumpSpeedMap(SWDkHzToSpeedMap[:])
			h.SetSpeed(initialInterfaceSpeed, false)
		}
	}

	if h.version.jtag_api == STLINK_JTAG_API_V3 {
		var smap = make([]speedMap, STLINK_V3_MAX_FREQ_NB)

		h.usbGetComFreq(stLinkMode == STLINK_MODE_DEBUG_JTAG, &smap)
		dumpSpeedMap(smap)
		h.SetSpeed(initialInterfaceSpeed, false)
	}

	// preliminary SRST assert:
	//  We want SRST is asserted before activating debug signals (mode_enter).
	//  As the required mode has not been set, the adapter may not know what pin to use.
	//  Tested firmware STLINK v2 JTAG v29 API v2 SWIM v0 uses T_NRST pin by default
	//  Tested firmware STLINK v2 JTAG v27 API v2 SWIM v6 uses T_NRST pin by default
	//  after power on, SWIM_RST stays unchanged
	if connectUnderReset && stLinkMode != STLINK_MODE_DEBUG_SWIM {
		h.usbAssertSrst(0)
		// do not check the return status here, we will
		//   proceed and enter the desired mode below
		//   and try asserting srst again.
	}

	err = h.usbModeEnter(stLinkMode)

	if err != nil {
		return err
	}

	if connectUnderReset {
		err = h.usbAssertSrst(0)
		if err != nil {
			return err
		}
	}

	mode, err = h.usbCurrentMode()

	if err != nil {
		return err
	}

	log.Debugf("Mode: 0x%02x", mode)

	return nil
}

func (h *StLinkHandle) usbLeaveMode(mode StLinkMode) error {
	h.usbInitBuffer(h.rx_ep, 0)

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
		return errors.New("unknown stlink mode")
	default:
		return errors.New("unknown stlink mode")
	}

	err := h.usbTransferNoErrCheck(h.databuf, 0)

	return err
}
