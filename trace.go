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

	log "github.com/sirupsen/logrus"
)

/**
 * @file
 * Holds the interface to TPIU, ITM and DWT configuration functions.
 */

type TraceConfigType int

const (
	TraceConfigTypeDisabled TraceConfigType = 0 /**< tracing is disabled */
	TraceConfigTypeExternal                 = 1 /**< trace output is captured externally */
	TraceConfigTypeInternal                 = 2 /**< trace output is handled by OpenOCD adapter driver */
)

type TpuiPinProtocolType int

const (
	TpuiPinProtocolSync           TpuiPinProtocolType = 0 /**< synchronous trace output */
	TpuiPinProtocolAsynManchester                     = 1 /**< asynchronous output with Manchester coding */
	TpuiPinProtocolAsyncUart                          = 2 /**< asynchronous output with NRZ coding */
)

type ItmTsPrescaler int

const (
	ItmTsPrescale1  ItmTsPrescaler = 0 /**< no prescaling for the timestamp counter */
	ItmTsPrescale4                 = 1 /**< refclock divided by 4 for the timestamp counter */
	ItmTsPrescale16                = 2 /**< refclock divided by 16 for the timestamp counter */
	ItmTsPrescale64                = 3 /**< refclock divided by 64 for the timestamp counter */
)

const TpuiAcprMaxSwoScaler = 0x1fff

func (h *StLinkHandle) usbTraceDisable() error {

	if h.version.flags&STLINK_F_HAS_TRACE == 0 {
		return errors.New("stlink does not support trace")
	}

	log.Debug("tracing: disable")

	h.usbInitBuffer(h.rx_ep, 2)
	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_STOP_TRACE_RX

	err := h.usbTransferErrCheck(h.databuf, 2)

	if err == nil {
		h.trace.enabled = false
		return nil
	} else {
		return errors.New("could not disable trace")
	}
}

func (h *StLinkHandle) usbTraceEnable() error {

	if h.version.flags&STLINK_F_HAS_TRACE != 0 {
		h.usbInitBuffer(h.rx_ep, 10)

		h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
		h.cmdidx++
		h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_START_TRACE_RX
		h.cmdidx++

		uint16ToLittleEndian(h.cmdbuf[h.cmdidx:], STLINK_TRACE_SIZE)
		h.cmdidx += 2

		uint32ToLittleEndian(h.cmdbuf[h.cmdidx:], h.trace.sourceHz)
		h.cmdidx += 4

		err := h.usbTransferErrCheck(h.databuf, 2)

		if err == nil {
			h.trace.enabled = true
			log.Debugf("Tracing: recording at %d Hz", h.trace.sourceHz)

			return nil
		} else {
			return errors.New("Error during usb xfer ")
		}
	} else {
		return errors.New("tracing not supported by this version")
	}
}

func (h *StLinkHandle) usbReadTrace(buffer []byte, size uint32) error {
	if h.version.flags&STLINK_F_HAS_TRACE == 0 {
		return errors.New("trace is not active")
	}

	inP, err := h.usb_interface.InEndpoint(int(h.trace_ep))

	if err != nil {
		return errors.New(fmt.Sprintf("could not open in endpoint #%d for trace output", int(h.trace_ep)))
	}

	bytesRead, err := usbRead(inP, buffer)

	if err != nil {
		return err
	} else {
		log.Debugf("Read trace %d of %d", bytesRead, size)
		return nil
	}
}
