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
	if h.version.jtagApi != jTagApiV1 {
		rxSize = 2
	}

	ctx := h.initTransfer(transferRxEndpoint)

	switch stMode {
	case StLinkModeDebugJtag:
		ctx.cmdBuffer.WriteByte(cmdDebug)

		if h.version.jtagApi == jTagApiV1 {
			ctx.cmdBuffer.WriteByte(debugApiV1Enter)
		} else {
			ctx.cmdBuffer.WriteByte(debugApiV2Enter)
		}

		ctx.cmdBuffer.WriteByte(debugEnterJTagNoReset)

	case StLinkModeDebugSwd:
		ctx.cmdBuffer.WriteByte(cmdDebug)

		if h.version.jtagApi == jTagApiV1 {
			ctx.cmdBuffer.WriteByte(debugApiV1Enter)
		} else {
			ctx.cmdBuffer.WriteByte(debugApiV2Enter)
		}

		ctx.cmdBuffer.WriteByte(debugEnterSwdNoReset)

	case StLinkModeDebugSwim:
		ctx.cmdBuffer.WriteByte(cmdSwim)
		ctx.cmdBuffer.WriteByte(swimEnter)

		/* swim enter does not return any response or status */
		return h.usbTransferNoErrCheck(ctx, 0)
	case StLinkModeDfu:
	case StLinkModeMass:
	default:
		return errors.New("cannot set usb mode from DFU or mass stlink configuration")
	}

	return h.usbCmdAllowRetry(ctx, rxSize)
}

func (h *StLinkHandle) usbCurrentMode() (byte, error) {

	ctx := h.initTransfer(transferRxEndpoint)

	ctx.cmdBuffer.WriteByte(cmdGetCurrentMode)

	err := h.usbTransferNoErrCheck(ctx, 2)

	if err != nil {
		return 0, err
	} else {
		return ctx.dataBuffer.Bytes()[0], nil
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
	case deviceModeDFU:
		stLinkMode = StLinkModeDfu

	case deviceModeDebug:
		stLinkMode = StLinkModeDebugSwd

	case deviceModeSwim:
		stLinkMode = StLinkModeDebugSwim

	case deviceModeBootloader, deviceModeMass:
		stLinkMode = StLinkModeUnknown

	default:
		stLinkMode = StLinkModeUnknown
	}

	if stLinkMode != StLinkModeUnknown {
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
	if mode != deviceModeDFU {
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

	stLinkMode = h.stMode

	if stLinkMode == StLinkModeUnknown {
		return errors.New("Selected mode (transport) not supported")
	}

	if stLinkMode == StLinkModeDebugJtag {
		if (h.version.flags & flagHasJtagSetFreq) != 0 {
			dumpSpeedMap(jTAGkHzToSpeedMap[:])
			h.SetSpeed(initialInterfaceSpeed, false)
		}
	} else if stLinkMode == StLinkModeDebugSwd {
		if (h.version.flags & flagHasJtagSetFreq) != 0 {
			dumpSpeedMap(swdKHzToSpeedMap[:])
			h.SetSpeed(initialInterfaceSpeed, false)
		}
	}

	if h.version.jtagApi == jTagApiV3 {
		var smap = make([]speedMap, v3MaxFreqNb)

		h.usbGetComFreq(stLinkMode == StLinkModeDebugJtag, &smap)
		dumpSpeedMap(smap)
		h.SetSpeed(initialInterfaceSpeed, false)
	}

	// preliminary SRST assert:
	//  We want SRST is asserted before activating debug signals (mode_enter).
	//  As the required mode has not been set, the adapter may not know what pin to use.
	//  Tested firmware STLINK v2 JTAG v29 API v2 SWIM v0 uses T_NRST pin by default
	//  Tested firmware STLINK v2 JTAG v27 API v2 SWIM v6 uses T_NRST pin by default
	//  after power on, SWIM_RST stays unchanged
	if connectUnderReset && stLinkMode != StLinkModeDebugSwim {
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
	ctx := h.initTransfer(transferRxEndpoint)

	switch mode {
	case StLinkModeDebugJtag, StLinkModeDebugSwd:
		ctx.cmdBuffer.WriteByte(cmdDebug)
		ctx.cmdBuffer.WriteByte(debugExit)

	case StLinkModeDebugSwim:
		ctx.cmdBuffer.WriteByte(cmdSwim)
		ctx.cmdBuffer.WriteByte(swimExit)

	case StLinkModeDfu:
		ctx.cmdBuffer.WriteByte(cmdDfu)
		ctx.cmdBuffer.WriteByte(dfuExit)

	case StLinkModeMass:
		return errors.New("unknown stlink mode")
	default:
		return errors.New("unknown stlink mode")
	}

	err := h.usbTransferNoErrCheck(ctx, 0)

	return err
}
