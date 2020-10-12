// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"bytes"
	"errors"
	"fmt"
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
		err := h.usbTransferV1GetStatus(ctx)

		if err != nil {
			return err
		}
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

func (h *StLink) usbTransferV1CreateCmd(ctx *transferCtx, cmdLength uint32) {
	ctx.cmdBuffer.WriteByte('U')
	ctx.cmdBuffer.WriteByte('S')
	ctx.cmdBuffer.WriteByte('B')
	ctx.cmdBuffer.WriteByte('C')

	addU32ToBuffer(&ctx.cmdBuffer, 0, 32, 0)
	addU32ToBuffer(&ctx.cmdBuffer, 0, 32, cmdLength)

	/* cbw flags */
	if ctx.direction == transferRxEndpoint {
		ctx.cmdBuffer.WriteByte(usbEndpointIn)
	} else {
		ctx.cmdBuffer.WriteByte(usbEndpointOut)
	}

	ctx.cmdBuffer.WriteByte(0) /* lun */
	ctx.cmdBuffer.WriteByte(0) /* cdb clength (is filled in at xfer) */
}

func (h *StLink) usbTransferV1GetStatus(ctx *transferCtx) error {
	ctx.cmdBuffer.Truncate(0)

	bytesRead, err := usbRead(h.rxEndpoint, ctx.cmdBuffer.Bytes())

	if err != nil || bytesRead != 13 {
		return errors.New("st-link V1 status read error")
	}

	t1 := buf_get_u32(ctx.cmdBuffer.Bytes(), 0, 32)

	/* check for USBS */
	if t1 != 0x53425355 {
		return errors.New("st-link usbs check error")
	}

	/*
	 * CSW status:
	 * 0 success
	 * 1 command failure
	 * 2 phase error
	 */
	if ctx.cmdBuffer.Bytes()[12] != 0 {
		return errors.New(fmt.Sprintf("got CSW status error %d", ctx.cmdBuffer.Bytes()[12]))
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
