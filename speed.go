package gostlink

import (
	"errors"
	"fmt"
	"math"

	log "github.com/sirupsen/logrus"
)

/* SWD clock speed */
type speed_map struct {
	speed         int
	speed_divisor int
}

var stlink_khz_to_speed_map_swd = [...]speed_map{
	{4000, 0},
	{1800, 1}, /* default */
	{1200, 2},
	{950, 3},
	{480, 7},
	{240, 15},
	{125, 31},
	{100, 40},
	{50, 79},
	{25, 158},
	{15, 265},
	{5, 798},
}

/* JTAG clock speed */
var stlink_khz_to_speed_map_jtag = [...]speed_map{
	{9000, 4},
	{4500, 8},
	{2250, 16},
	{1125, 32}, /* default */
	{562, 64},
	{281, 128},
	{140, 256},
}

func (h *StLinkHandle) set_speed_v3(is_jtag bool, khz int, query bool) (int, error) {

	var smap = make([]speed_map, STLINK_V3_MAX_FREQ_NB)

	h.usb_get_com_freq(is_jtag, &smap)

	speed_index, err := stlink_match_speed_map(smap, khz, query)

	if err != nil {
		return khz, err
	}

	if !query {
		err := h.usb_set_com_freq(is_jtag, smap[speed_index].speed)

		if err != nil {
			return khz, err
		}
	}

	return smap[speed_index].speed, nil
}

func (h *StLinkHandle) set_speed_swd(khz int, query bool) (int, error) {

	/* old firmware cannot change it */
	if (h.version.flags & STLINK_F_HAS_SWD_SET_FREQ) == 0 {
		return khz, errors.New("Cannot change speed on old firmware")
	}

	speed_index, err := stlink_match_speed_map(stlink_khz_to_speed_map_swd[:], khz, query)

	if err != nil {
		return khz, err
	}

	if !query {
		error := h.usb_set_swdclk(uint16(stlink_khz_to_speed_map_swd[speed_index].speed_divisor))

		if error != nil {
			return khz, errors.New("Unable to set adapter speed")
		}
	}

	return stlink_khz_to_speed_map_swd[speed_index].speed, nil
}

func (h *StLinkHandle) usb_set_swdclk(clk_divisor uint16) error {

	if (h.version.flags & STLINK_F_HAS_SWD_SET_FREQ) == 0 {
		errors.New("Cannot change speed on this firmware")
	}

	h.usb_init_buffer(h.rx_ep, 2)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_SWD_SET_FREQ
	h.cmdidx++

	h_u16_to_le(h.cmdbuf[h.cmdidx:], int(clk_divisor))
	h.cmdidx += 2

	err := h.usb_cmd_allow_retry(h.databuf, 2)

	return err
}

func (h *StLinkHandle) usb_get_com_freq(is_jtag bool, smap *[]speed_map) error {

	if h.version.jtag_api != STLINK_JTAG_API_V3 {
		return errors.New("Unknown command")
	}

	h.usb_init_buffer(h.rx_ep, 16)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = STLINK_APIV3_GET_COM_FREQ
	h.cmdidx++

	if is_jtag {
		h.cmdbuf[h.cmdidx] = 1
	} else {
		h.cmdbuf[h.cmdidx] = 0
	}
	h.cmdidx++

	err := h.usb_xfer_errcheck(h.databuf, 52)

	var size int = int(h.databuf[8])

	if size > STLINK_V3_MAX_FREQ_NB {
		size = STLINK_V3_MAX_FREQ_NB
	}

	for i := 0; i < size; i++ {
		(*smap)[i].speed = int(le_to_h_u32(h.databuf[12+4*i:]))
		(*smap)[i].speed_divisor = i
	}

	// set to zero all the next entries
	for i := size; i < STLINK_V3_MAX_FREQ_NB; i++ {
		(*smap)[i].speed = 0
	}

	if err == ERROR_OK {
		return nil
	} else {
		return errors.New("Got error check fail")
	}
}

func (h *StLinkHandle) usb_set_com_freq(is_jtag bool, frequency int) error {

	if h.version.jtag_api != STLINK_JTAG_API_V3 {
		return errors.New("Unknown command")
	}

	h.usb_init_buffer(h.rx_ep, 16)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = STLINK_APIV3_SET_COM_FREQ
	h.cmdidx++

	if is_jtag {
		h.cmdbuf[h.cmdidx] = 1
	} else {
		h.cmdbuf[h.cmdidx] = 0
	}
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++

	h_u32_to_le(h.cmdbuf[4:], frequency)

	err := h.usb_xfer_errcheck(h.databuf, 8)

	if err == ERROR_OK {
		return nil
	} else {
		return errors.New("Got error check fail")
	}
}

func stlink_match_speed_map(smap []speed_map, khz int, query bool) (int, error) {
	var last_valid_speed int = -1
	var speed_index = -1
	var speed_diff = math.MaxInt32
	var match bool = false
	var counter int = 0

	for i, s := range smap {
		counter = i
		if s.speed == 0 {
			continue
		}

		last_valid_speed = i
		if khz == s.speed {
			speed_index = i
			break
		} else {
			var current_diff = khz - s.speed

			//get abs value for comparison
			if current_diff <= 0 {
				current_diff = -current_diff
			}

			if (current_diff < speed_diff) && khz >= s.speed {
				speed_diff = current_diff
				speed_index = i
			}
		}
	}

	if speed_index == -1 {
		// this will only be here if we cannot match the slow speed.
		// use the slowest speed we support.
		speed_index = last_valid_speed
		match = false
	} else if counter == len(smap) {
		match = false
	}

	if !match && query {
		return -1, errors.New(fmt.Sprintf("Unable to match requested speed %d kHz, using %d kHz",
			khz, smap[speed_index].speed))
	}

	return speed_index, nil
}

func stlink_dump_speed_map(smap []speed_map) {
	for i := range smap {
		if smap[i].speed > 0 {
			log.Debugf("%d kHz", smap[i].speed)
		}
	}
}
