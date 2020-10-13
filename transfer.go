// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"bytes"
	"errors"
	"time"
)

type transferCtx struct {
	cmdBuffer  bytes.Buffer
	dataBuffer bytes.Buffer

	direction usbTransferEndpoint

	cmdSize uint32
}

func (h *StLink) initTransfer(endpoint usbTransferEndpoint) *transferCtx {
	context := &transferCtx{}

	context.direction = endpoint
	context.cmdBuffer.Grow(cmdBufferSize)
	context.dataBuffer.Grow(dataBufferSize)
	context.cmdSize = 0

	return context
}

func (h *StLink) usbTransferNoErrCheck(ctx *transferCtx, dataLength uint32) error {
	ctx.cmdSize = cmdSizeV2

	if h.version.stlink == 1 {
		ctx.cmdSize = cmdBufferSize
		ctx.cmdBuffer.Bytes()[14] = uint8(ctx.cmdBuffer.Len() - 15)
	}

	err := h.usbTransferReadWrite(ctx, dataLength)

	if err != nil {
		return err
	}

	if h.version.stlink == 1 {
		return errors.New("st-link V1 api commands not supported")
	}

	return nil
}

func (h *StLink) usbTransferErrCheck(ctx *transferCtx, dataLength uint32) error {

	err := h.usbTransferNoErrCheck(ctx, dataLength)

	if err != nil {
		return err
	}

	return h.usbErrorCheck(ctx)
}

func (h *StLink) usbTransferReadWrite(ctx *transferCtx, dataLength uint32) error {

	_, err := usbWrite(h.txEndpoint, ctx.cmdBuffer.Bytes()[:ctx.cmdSize])

	if err != nil {
		return err
	}

	if ctx.direction == transferTxEndpoint && dataLength > 0 {

		time.Sleep(time.Millisecond * 10)

		_, err = usbWrite(h.txEndpoint, ctx.dataBuffer.Bytes()[:dataLength])

		if err != nil {
			return err
		}

	} else if h.transferEndpoint == transferRxEndpoint && dataLength > 0 {

		readBuffer := make([]byte, dataLength)

		_, err = usbRead(h.rxEndpoint, readBuffer)

		if err != nil {
			return err
		}

		ctx.dataBuffer.Write(readBuffer)
	}

	return nil
}

func (h *StLink) usbGetReadWriteStatus() error {

	if h.version.jtagApi == jTagApiV1 {
		return nil
	}

	ctx := h.initTransfer(transferRxEndpoint)
	ctx.cmdBuffer.WriteByte(cmdDebug)

	if h.version.flags.Get(flagHasGetLastRwStatus2) {
		ctx.cmdBuffer.WriteByte(debugApiV2GetLastRWStatus2)

		return h.usbTransferErrCheck(ctx, 12)

	} else {
		ctx.cmdBuffer.WriteByte(debugApiV2GetLastRWStatus)

		return h.usbTransferErrCheck(ctx, 2)
	}
}
