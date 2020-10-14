// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"errors"
	"time"
)

type transferCtx struct {
	cmdBuf  *Buffer
	dataBuf *Buffer

	direction usbTransferDirection

	cmdSize uint32
}

func (t *transferCtx) CmdBytes() []byte {
	return t.cmdBuf.Bytes()
}

func (t *transferCtx) DataBytes() []byte {
	return t.dataBuf.Bytes()
}

func (h *StLink) initTransfer(dir usbTransferDirection) *transferCtx {
	ctx := &transferCtx{cmdSize: 0}

	ctx.cmdBuf = NewBuffer(cmdBufferSize)
	ctx.dataBuf = NewBuffer(dataBufferSize)

	ctx.direction = dir

	return ctx
}

func (h *StLink) usbTransferErrCheck(ctx *transferCtx, dataLength uint32) error {

	err := h.usbTransferNoErrCheck(ctx, dataLength)

	if err != nil {
		logger.Error("during usb transfer with error check ", err)
		return err
	}

	return h.usbErrorCheck(ctx)
}

func (h *StLink) usbTransferNoErrCheck(ctx *transferCtx, dataLength uint32) error {
	ctx.cmdSize = cmdSizeV2

	if h.version.stlink == 1 {
		return errors.New("st-link V1 api commands not supported")
	}

	return h.usbTransferReadWrite(ctx, dataLength)
}

func (h *StLink) usbTransferReadWrite(ctx *transferCtx, dataLength uint32) error {

	_, err := usbRawWrite(h.txEndpoint, ctx.cmdBuf.Bytes()[:ctx.cmdSize])

	if err != nil {
		return err
	}

	if ctx.direction == transferOutgoing && dataLength > 0 {

		time.Sleep(time.Millisecond * 10)

		_, err = usbRawWrite(h.txEndpoint, ctx.dataBuf.Bytes()[:dataLength])

		if err != nil {
			return err
		}

	} else if ctx.direction == transferIncoming && dataLength > 0 {

		readBuffer := make([]byte, dataLength)

		_, err = usbRawRead(h.rxEndpoint, readBuffer)

		if err != nil {
			return err
		}

		ctx.dataBuf.Write(readBuffer)
	}

	return nil
}

func (h *StLink) usbGetReadWriteStatus() error {

	if h.version.jtagApi == jTagApiV1 {
		logger.Warn("get read write status not supported in jTag api V1")
		return nil
	}

	ctx := h.initTransfer(transferIncoming)
	ctx.cmdBuf.WriteByte(cmdDebug)

	if h.version.flags.Get(flagHasGetLastRwStatus2) {
		ctx.cmdBuf.WriteByte(debugApiV2GetLastRWStatus2)

		return h.usbTransferErrCheck(ctx, 12)

	} else {
		ctx.cmdBuf.WriteByte(debugApiV2GetLastRWStatus)

		return h.usbTransferErrCheck(ctx, 2)
	}
}
