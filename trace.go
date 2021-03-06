// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"errors"
)

type TraceConfigType int

const (
	TraceConfigTypeDisabled TraceConfigType = 0 // tracing is disabled
	TraceConfigTypeExternal                 = 1 // trace output is captured externally
	TraceConfigTypeInternal                 = 2 // trace output is handled by OpenOCD adapter driver
)

type TpuiPinProtocolType int

const (
	TpuiPinProtocolSync           TpuiPinProtocolType = 0 // synchronous trace output
	TpuiPinProtocolAsynManchester                     = 1 // asynchronous output with Manchester coding
	TpuiPinProtocolAsyncUart                          = 2 // asynchronous output with NRZ coding
)

type itmTsPrescaler int

const (
	itmTsPrescale1  itmTsPrescaler = 0 // no prescaling for the timestamp counter
	itmTsPrescale4                 = 1 // refclock divided by 4 for the timestamp counter
	itmTsPrescale16                = 2 // refclock divided by 16 for the timestamp counter
	itmTsPrescale64                = 3 // refclock divided by 64 for the timestamp counter
)

const tpuiAcprMaxSwoScaler = 0x1fff

func (h *StLink) usbTraceDisable() error {

	if !h.version.flags.Get(flagHasTrace) {
		return errors.New("st-link does not support trace")
	}

	ctx := h.initTransfer(transferIncoming)

	ctx.cmdBuf.WriteByte(cmdDebug)
	ctx.cmdBuf.WriteByte(debugApiV2StopTraceRx)

	err := h.usbTransferErrCheck(ctx, 2)

	if err == nil {
		h.trace.enabled = false
		return nil
	} else {
		return errors.New("could not disable trace")
	}
}

func (h *StLink) usbTraceEnable() error {

	if h.version.flags.Get(flagHasTrace) {
		ctx := h.initTransfer(transferIncoming)

		ctx.cmdBuf.WriteByte(cmdDebug)
		ctx.cmdBuf.WriteByte(debugApiV2StartTraceRx)

		ctx.cmdBuf.WriteUint16LE(traceSize)
		ctx.cmdBuf.WriteUint32LE(h.trace.sourceHz)

		err := h.usbTransferErrCheck(ctx, 2)

		if err == nil {
			h.trace.enabled = true
			logger.Debugf("enabled trace recording at %d Hz", h.trace.sourceHz)

			return nil
		} else {
			return errors.New("usb xfer error at enabling trace")
		}
	} else {
		return errors.New("tracing not supported by st-link")
	}
}

func (h *StLink) usbReadTrace(buffer []byte, size uint32) error {
	if !h.version.flags.Get(flagHasTrace) {
		return errors.New("trace is not supported by connected device")
	}

	bytesRead, err := usbRawRead(h.traceEndpoint, buffer)

	if err != nil {
		return err
	} else {
		logger.Debugf("Read [%d from %d] bytes from trace channel", bytesRead, size)
		return nil
	}
}
