// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"fmt"
)

type usbErrorCode int

const (
	usbErrorOK                    usbErrorCode = 0
	usbErrorWait                               = -1
	usbErrorFail                               = -2
	usbErrorTargetUnalignedAccess              = -3
	usbErrorCommandNotFound                    = -4
)

type usbError struct {
	errorString  string
	UsbErrorCode usbErrorCode
}

func (e *usbError) Error() string {
	return e.errorString
}

func newUsbError(msg string, code usbErrorCode) error {
	return &usbError{msg, code}
}

/**
  Converts an STLINK status code held in the first byte of a response
  to an gostlink library error, logs any error/wait status as debug output.
*/
func (h *StLink) usbErrorCheck(ctx *transferCtx) error {

	errorStatus := ctx.DataBytes()[0]

	if h.stMode == StLinkModeDebugSwim {
		switch errorStatus {
		case swimErrorOk:
			return nil

		case swimErrorBusy:
			return newUsbError("swim is busy", usbErrorWait)

		default:
			return newUsbError(fmt.Sprintf("unknown/unexpected STLINK status code 0x%x", errorStatus), usbErrorFail)
		}
	}

	/* TODO: no error checking yet on api V1 */
	if h.version.jtagApi == jTagApiV1 {
		errorStatus = debugErrorOk
	}

	switch errorStatus {
	case debugErrorOk:
		return nil

	case debugErrorFault:
		return newUsbError(fmt.Sprintf("SWD fault response (0x%x)", debugErrorFault), usbErrorFail)

	case swdAccessPortWait:
		return newUsbError(fmt.Sprintf("wait status SWD_AP_WAIT (0x%x)", swdAccessPortWait), usbErrorWait)

	case swdDebugPortWait:
		return newUsbError(fmt.Sprintf("wait status SWD_DP_WAIT (0x%x)", swdDebugPortWait), usbErrorWait)

	case jTagGetIdCodeError:
		return newUsbError("STLINK_JTAG_GET_IDCODE_ERROR", usbErrorFail)

	case jTagWriteError:
		return newUsbError("Write error", usbErrorFail)

	case jTagWriteVerifyError:
		return newUsbError("Write verify error, ignoring", usbErrorOK)

	case swdAccessPortFault:
		/* git://git.ac6.fr/openocd commit 657e3e885b9ee10
		 * returns ERROR_OK with the comment:
		 * Change in error status when reading outside RAM.
		 * This fix allows CDT plugin to visualize memory.
		 */
		return newUsbError("STLINK_SWD_AP_FAULT", usbErrorFail)

	case swdAccessPortError:
		return newUsbError("STLINK_SWD_AP_ERROR", usbErrorFail)

	case swdAccessPortParityError:
		return newUsbError("STLINK_SWD_AP_PARITY_ERROR", usbErrorFail)

	case swdDebugPortFault:
		return newUsbError("STLINK_SWD_DP_FAULT", usbErrorFail)

	case swdDebugPortError:
		return newUsbError("STLINK_SWD_DP_ERROR", usbErrorFail)

	case swdDebugPortParityError:
		return newUsbError("STLINK_SWD_DP_PARITY_ERROR", usbErrorFail)

	case swdAccessPortWDataError:
		return newUsbError("STLINK_SWD_AP_WDATA_ERROR", usbErrorFail)

	case swdAccessPortStickyError:
		return newUsbError("STLINK_SWD_AP_STICKY_ERROR", usbErrorFail)

	case swdAccessPortStickOrRunError:
		return newUsbError("STLINK_SWD_AP_STICKYORUN_ERROR", usbErrorFail)

	case badAccessPortError:
		return newUsbError("STLINK_BAD_AP_ERROR", usbErrorFail)

	default:
		return newUsbError(fmt.Sprintf("unknown/unexpected STLINK status code 0x%x", errorStatus), usbErrorFail)
	}
}
