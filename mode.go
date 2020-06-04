package gostlink

import (
	"errors"

	log "github.com/sirupsen/logrus"
)

/** */
func (h *StLinkHandle) usb_mode_enter(st_mode StLinkMode) error {
	var rx_size uint32 = 0
	/* on api V2 we are able the read the latest command
	 * status
	 * TODO: we need the test on api V1 too
	 */
	if h.version.jtag_api != STLINK_JTAG_API_V1 {
		rx_size = 2
	}

	h.usb_init_buffer(h.rx_ep, rx_size)

	switch st_mode {
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
		return h.usb_xfer_noerrcheck(h.databuf, 0)
	case STLINK_MODE_DFU:
	case STLINK_MODE_MASS:
	default:
		return errors.New("Cannot set usb mode from DFU or mass stlink configuration")
	}

	return h.usb_cmd_allow_retry(h.databuf, int(rx_size))
}

func (h *StLinkHandle) usb_current_mode() (byte, error) {

	h.usb_init_buffer(h.rx_ep, 2)

	h.cmdbuf[h.cmdidx] = STLINK_GET_CURRENT_MODE
	h.cmdidx++

	err := h.usb_xfer_noerrcheck(h.databuf, 2)

	if err != nil {
		return 0, err
	} else {
		return h.databuf[0], nil
	}
}

func (h *StLinkHandle) usb_init_mode(connect_under_reset bool, initial_interface_speed int) error {

	mode, err := h.usb_current_mode()

	if err != nil {
		log.Error("Could not get usb mode")
		return err
	}

	log.Debugf("Got usb mode: %d", mode)

	var stlink_mode StLinkMode

	switch mode {
	case STLINK_DEV_DFU_MODE:
		stlink_mode = STLINK_MODE_DFU

	case STLINK_DEV_DEBUG_MODE:
		stlink_mode = STLINK_MODE_DEBUG_SWD

	case STLINK_DEV_SWIM_MODE:
		stlink_mode = STLINK_MODE_DEBUG_SWIM

	case STLINK_DEV_BOOTLOADER_MODE, STLINK_DEV_MASS_MODE:
		stlink_mode = STLINK_MODE_UNKNOWN
	default:
		stlink_mode = STLINK_MODE_UNKNOWN
	}

	if stlink_mode != STLINK_MODE_UNKNOWN {
		h.usb_leave_mode(stlink_mode)
	}

	mode, err = h.usb_current_mode()

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

	stlink_mode = h.st_mode

	if stlink_mode == STLINK_MODE_UNKNOWN {
		return errors.New("Selected mode (transport) not supported")
	}

	if stlink_mode == STLINK_MODE_DEBUG_JTAG {
		if (h.version.flags & STLINK_F_HAS_JTAG_SET_FREQ) != 0 {
			stlink_dump_speed_map(stlink_khz_to_speed_map_jtag[:])
			h.stlink_speed(initial_interface_speed, false)
		}
	} else if stlink_mode == STLINK_MODE_DEBUG_SWD {
		if (h.version.flags & STLINK_F_HAS_JTAG_SET_FREQ) != 0 {
			stlink_dump_speed_map(stlink_khz_to_speed_map_swd[:])
			h.stlink_speed(initial_interface_speed, false)
		}
	}

	if h.version.jtag_api == STLINK_JTAG_API_V3 {
		var smap = make([]speed_map, STLINK_V3_MAX_FREQ_NB)

		h.usb_get_com_freq(stlink_mode == STLINK_MODE_DEBUG_JTAG, &smap)
		stlink_dump_speed_map(smap)
		h.stlink_speed(initial_interface_speed, false)
	}

	// preliminary SRST assert:
	//  We want SRST is asserted before activating debug signals (mode_enter).
	//  As the required mode has not been set, the adapter may not know what pin to use.
	//  Tested firmware STLINK v2 JTAG v29 API v2 SWIM v0 uses T_NRST pin by default
	//  Tested firmware STLINK v2 JTAG v27 API v2 SWIM v6 uses T_NRST pin by default
	//  after power on, SWIM_RST stays unchanged
	if connect_under_reset && stlink_mode != STLINK_MODE_DEBUG_SWIM {
		h.usb_assert_srst(0)
		// do not check the return status here, we will
		//   proceed and enter the desired mode below
		//   and try asserting srst again.
	}

	err = h.usb_mode_enter(stlink_mode)

	if err != nil {
		return err
	}

	if connect_under_reset {
		err = h.usb_assert_srst(0)
		if err != nil {
			return err
		}
	}

	mode, err = h.usb_current_mode()

	if err != nil {
		return err
	}

	log.Debugf("Mode: 0x%02x", mode)

	return nil
}

func (h *StLinkHandle) usb_leave_mode(mode StLinkMode) error {
	h.usb_init_buffer(h.rx_ep, 0)

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
		return errors.New("Unknown stlink mode")
	default:
		return errors.New("Unknown stlink mode")
	}

	err := h.usb_xfer_noerrcheck(h.databuf, 0)

	return err
}
