// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

// this code is mainly inspired and based on the openocd project source code
// for detailed information see

// https://sourceforge.net/p/openocd/code

package gostlink

import (
	"errors"
	"fmt"
	"math"

	log "github.com/sirupsen/logrus"
)

/* SWD clock speed */
type speedMap struct {
	speed        uint32
	speedDivisor uint32
}

var SWDkHzToSpeedMap = [...]speedMap{
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
var JTAGkHzToSpeedMap = [...]speedMap{
	{9000, 4},
	{4500, 8},
	{2250, 16},
	{1125, 32}, /* default */
	{562, 64},
	{281, 128},
	{140, 256},
}

func (h *StLinkHandle) setSpeedV3(isJtag bool, kHz uint32, querySpeed bool) (uint32, error) {

	var smap = make([]speedMap, STLINK_V3_MAX_FREQ_NB)

	h.usbGetComFreq(isJtag, &smap)

	speedIndex, err := matchSpeedMap(smap, kHz, querySpeed)

	if err != nil {
		return kHz, err
	}

	if !querySpeed {
		err := h.usbSetComFreq(isJtag, smap[speedIndex].speed)

		if err != nil {
			return kHz, err
		}
	}

	return smap[speedIndex].speed, nil
}

func (h *StLinkHandle) setSpeedSwd(kHz uint32, querySpeed bool) (uint32, error) {
	/* old firmware cannot change it */
	if (h.version.flags & STLINK_F_HAS_SWD_SET_FREQ) == 0 {
		return kHz, errors.New("target st-link doesn't support swd speed change")
	}

	speedIndex, err := matchSpeedMap(SWDkHzToSpeedMap[:], kHz, querySpeed)

	if err != nil {
		return kHz, err
	}

	if !querySpeed {
		error := h.usbSetSwdClk(uint16(SWDkHzToSpeedMap[speedIndex].speedDivisor))

		if error != nil {
			return kHz, errors.New("could not set swd clock speed")
		}
	}

	return SWDkHzToSpeedMap[speedIndex].speed, nil
}

func matchSpeedMap(smap []speedMap, kHz uint32, query bool) (int, error) {
	var lastValidSpeed int = -1
	var speedIndex = -1
	var speedDiff uint32 = math.MaxUint32
	var match bool = true
	var counter int = 0

	for i, s := range smap {
		counter = i
		if s.speed == 0 {
			continue
		}

		lastValidSpeed = i

		if kHz == s.speed {
			speedIndex = i
			break
		} else {
			var currentDiff = kHz - s.speed

			//get abs value for comparison
			if currentDiff <= 0 {
				currentDiff = -currentDiff
			}

			if (currentDiff < speedDiff) && kHz >= s.speed {
				speedDiff = currentDiff
				speedIndex = i
			}
		}
	}

	if speedIndex == -1 {
		// this will only be here if we cannot match the slow speed.
		// use the slowest speed we support.
		speedIndex = lastValidSpeed
		match = false
	} else if counter == len(smap) {
		match = false
	}

	if !match && query {
		return -1, errors.New(fmt.Sprintf("Unable to match requested speed %d kHz, using %d kHz",
			kHz, smap[speedIndex].speed))
	}

	return speedIndex, nil
}

func dumpSpeedMap(smap []speedMap) {
	for i := range smap {
		if smap[i].speed > 0 {
			log.Debugf("%d kHz", smap[i].speed)
		}
	}
}

func (h *StLinkHandle) usbSetSwdClk(clkDivisor uint16) error {

	if (h.version.flags & STLINK_F_HAS_SWD_SET_FREQ) == 0 {
		errors.New("cannot change speed on this firmware")
	}

	h.usbInitBuffer(h.rx_ep, 2)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_SWD_SET_FREQ
	h.cmdidx++

	uint16ToLittleEndian(h.cmdbuf[h.cmdidx:], clkDivisor)
	h.cmdidx += 2

	err := h.usbCmdAllowRetry(h.databuf, 2)

	return err
}

func (h *StLinkHandle) usbGetComFreq(isJtag bool, smap *[]speedMap) error {

	if h.version.jtag_api != STLINK_JTAG_API_V3 {
		return errors.New("Unknown command")
	}

	h.usbInitBuffer(h.rx_ep, 16)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = STLINK_APIV3_GET_COM_FREQ
	h.cmdidx++

	if isJtag {
		h.cmdbuf[h.cmdidx] = 1
	} else {
		h.cmdbuf[h.cmdidx] = 0
	}
	h.cmdidx++

	err := h.usbTransferErrCheck(h.databuf, 52)

	size := uint32(h.databuf[8])

	if size > STLINK_V3_MAX_FREQ_NB {
		size = STLINK_V3_MAX_FREQ_NB
	}

	for i := uint32(0); i < size; i++ {
		(*smap)[i].speed = le_to_h_u32(h.databuf[12+4*i:])
		(*smap)[i].speedDivisor = i
	}

	// set to zero all the next entries
	for i := size; i < STLINK_V3_MAX_FREQ_NB; i++ {
		(*smap)[i].speed = 0
	}

	return err
}

func (h *StLinkHandle) usbSetComFreq(isJtag bool, frequency uint32) error {

	if h.version.jtag_api != STLINK_JTAG_API_V3 {
		return errors.New("unknown command")
	}

	h.usbInitBuffer(h.rx_ep, 16)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = STLINK_APIV3_SET_COM_FREQ
	h.cmdidx++

	if isJtag {
		h.cmdbuf[h.cmdidx] = 1
	} else {
		h.cmdbuf[h.cmdidx] = 0
	}
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++

	uint32ToLittleEndian(h.cmdbuf[4:], frequency)

	err := h.usbTransferErrCheck(h.databuf, 8)

	return err
}
