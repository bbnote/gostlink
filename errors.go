package gostlink

import log "github.com/sirupsen/logrus"

const (
	ERROR_OK                      = 0
	ERROR_WAIT                    = -1
	ERROR_FAIL                    = -2
	ERROR_TARGET_UNALIGNED_ACCESS = -3
	ERROR_COMMAND_NOTFOUND        = -4
)

/**
  Converts an STLINK status code held in the first byte of a response
  to an openocd error, logs any error/wait status as debug output.
*/
func (h *StLinkHandle) usb_error_check() int {

	if h.st_mode == STLINK_MODE_DEBUG_SWIM {
		switch h.databuf[0] {
		case STLINK_SWIM_ERR_OK:
			return ERROR_OK

		case STLINK_SWIM_BUSY:
			log.Debug("SWIM Busy")
			return ERROR_WAIT

		default:
			log.Debugf("unknown/unexpected STLINK status code 0x%x", h.databuf[0])
			return ERROR_FAIL
		}
	}

	/* TODO: no error checking yet on api V1 */
	if h.version.jtag_api == STLINK_JTAG_API_V1 {
		h.databuf[0] = STLINK_DEBUG_ERR_OK
	}

	switch h.databuf[0] {
	case STLINK_DEBUG_ERR_OK:
		return ERROR_OK

	case STLINK_DEBUG_ERR_FAULT:
		log.Debugf("SWD fault response (0x%x)", STLINK_DEBUG_ERR_FAULT)
		return ERROR_FAIL
	case STLINK_SWD_AP_WAIT:
		log.Debugf("wait status SWD_AP_WAIT (0x%x)", STLINK_SWD_AP_WAIT)
		return ERROR_WAIT
	case STLINK_SWD_DP_WAIT:
		log.Debugf("wait status SWD_DP_WAIT (0x%x)", STLINK_SWD_DP_WAIT)
		return ERROR_WAIT
	case STLINK_JTAG_GET_IDCODE_ERROR:
		log.Debug("STLINK_JTAG_GET_IDCODE_ERROR")
		return ERROR_FAIL
	case STLINK_JTAG_WRITE_ERROR:
		log.Debug("Write error")
		return ERROR_FAIL
	case STLINK_JTAG_WRITE_VERIF_ERROR:
		log.Debug("Write verify error, ignoring")
		return ERROR_OK
	case STLINK_SWD_AP_FAULT:
		/* git://git.ac6.fr/openocd commit 657e3e885b9ee10
		 * returns ERROR_OK with the comment:
		 * Change in error status when reading outside RAM.
		 * This fix allows CDT plugin to visualize memory.
		 */
		log.Debug("STLINK_SWD_AP_FAULT")
		return ERROR_FAIL
	case STLINK_SWD_AP_ERROR:
		log.Debug("STLINK_SWD_AP_ERROR")
		return ERROR_FAIL
	case STLINK_SWD_AP_PARITY_ERROR:
		log.Debug("STLINK_SWD_AP_PARITY_ERROR")
		return ERROR_FAIL
	case STLINK_SWD_DP_FAULT:
		log.Debug("STLINK_SWD_DP_FAULT")
		return ERROR_FAIL
	case STLINK_SWD_DP_ERROR:
		log.Debug("STLINK_SWD_DP_ERROR")
		return ERROR_FAIL
	case STLINK_SWD_DP_PARITY_ERROR:
		log.Debug("STLINK_SWD_DP_PARITY_ERROR")
		return ERROR_FAIL
	case STLINK_SWD_AP_WDATA_ERROR:
		log.Debug("STLINK_SWD_AP_WDATA_ERROR")
		return ERROR_FAIL
	case STLINK_SWD_AP_STICKY_ERROR:
		log.Debug("STLINK_SWD_AP_STICKY_ERROR")
		return ERROR_FAIL
	case STLINK_SWD_AP_STICKYORUN_ERROR:
		log.Debug("STLINK_SWD_AP_STICKYORUN_ERROR")
		return ERROR_FAIL
	case STLINK_BAD_AP_ERROR:
		log.Debug("STLINK_BAD_AP_ERROR")
		return ERROR_FAIL
	default:
		log.Debugf("unknown/unexpected STLINK status code 0x%x", h.databuf[0])
		return ERROR_FAIL
	}
}
