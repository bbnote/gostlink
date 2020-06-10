package gostlink

import (
	"fmt"
)

type UsbErrorCode int

const (
	ErrorOK                    UsbErrorCode = 0
	ErrorWait                               = -1
	ErrorFail                               = -2
	ErrorTargetUnalignedAccess              = -3
	ErrorCommandNotFound                    = -4
)

type UsbError struct {
	errorString  string
	UsbErrorCode UsbErrorCode
}

func (e *UsbError) Error() string {
	return e.errorString
}

func NewUsbError(msg string, code UsbErrorCode) error {
	return &UsbError{msg, code}
}

/**
  Converts an STLINK status code held in the first byte of a response
  to an gostlink library error, logs any error/wait status as debug output.
*/
func (h *StLinkHandle) usbErrorCheck() error {

	if h.st_mode == STLINK_MODE_DEBUG_SWIM {
		switch h.databuf[0] {
		case STLINK_SWIM_ERR_OK:
			return nil

		case STLINK_SWIM_BUSY:
			return NewUsbError("swim is busy", ErrorWait)

		default:
			return NewUsbError(fmt.Sprintf("unknown/unexpected STLINK status code 0x%x", h.databuf[0]), ErrorFail)
		}
	}

	/* TODO: no error checking yet on api V1 */
	if h.version.jtag_api == STLINK_JTAG_API_V1 {
		h.databuf[0] = STLINK_DEBUG_ERR_OK
	}

	switch h.databuf[0] {
	case STLINK_DEBUG_ERR_OK:
		return nil

	case STLINK_DEBUG_ERR_FAULT:
		return NewUsbError(fmt.Sprintf("SWD fault response (0x%x)", STLINK_DEBUG_ERR_FAULT), ErrorFail)

	case STLINK_SWD_AP_WAIT:
		return NewUsbError(fmt.Sprintf("wait status SWD_AP_WAIT (0x%x)", STLINK_SWD_AP_WAIT), ErrorWait)

	case STLINK_SWD_DP_WAIT:
		return NewUsbError(fmt.Sprintf("wait status SWD_DP_WAIT (0x%x)", STLINK_SWD_DP_WAIT), ErrorWait)

	case STLINK_JTAG_GET_IDCODE_ERROR:
		return NewUsbError("STLINK_JTAG_GET_IDCODE_ERROR", ErrorFail)

	case STLINK_JTAG_WRITE_ERROR:
		return NewUsbError("Write error", ErrorFail)

	case STLINK_JTAG_WRITE_VERIF_ERROR:
		return NewUsbError("Write verify error, ignoring", ErrorOK)

	case STLINK_SWD_AP_FAULT:
		/* git://git.ac6.fr/openocd commit 657e3e885b9ee10
		 * returns ERROR_OK with the comment:
		 * Change in error status when reading outside RAM.
		 * This fix allows CDT plugin to visualize memory.
		 */
		return NewUsbError("STLINK_SWD_AP_FAULT", ErrorFail)

	case STLINK_SWD_AP_ERROR:
		return NewUsbError("STLINK_SWD_AP_ERROR", ErrorFail)

	case STLINK_SWD_AP_PARITY_ERROR:
		return NewUsbError("STLINK_SWD_AP_PARITY_ERROR", ErrorFail)

	case STLINK_SWD_DP_FAULT:
		return NewUsbError("STLINK_SWD_DP_FAULT", ErrorFail)

	case STLINK_SWD_DP_ERROR:
		return NewUsbError("STLINK_SWD_DP_ERROR", ErrorFail)

	case STLINK_SWD_DP_PARITY_ERROR:
		return NewUsbError("STLINK_SWD_DP_PARITY_ERROR", ErrorFail)

	case STLINK_SWD_AP_WDATA_ERROR:
		return NewUsbError("STLINK_SWD_AP_WDATA_ERROR", ErrorFail)

	case STLINK_SWD_AP_STICKY_ERROR:
		return NewUsbError("STLINK_SWD_AP_STICKY_ERROR", ErrorFail)

	case STLINK_SWD_AP_STICKYORUN_ERROR:
		return NewUsbError("STLINK_SWD_AP_STICKYORUN_ERROR", ErrorFail)

	case STLINK_BAD_AP_ERROR:
		return NewUsbError("STLINK_BAD_AP_ERROR", ErrorFail)

	default:
		return NewUsbError(fmt.Sprintf("unknown/unexpected STLINK status code 0x%x", h.databuf[0]), ErrorFail)
	}
}
