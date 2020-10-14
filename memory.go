// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"bytes"
	"fmt"
)

func (h *StLink) usbReadMem8(addr uint32, len uint16, buffer *bytes.Buffer) error {
	var readLen = uint32(len)

	/* max 8 bit read/write is 64 bytes or 512 bytes for v3 */
	if readLen > h.usbBlock() {
		return newUsbError(fmt.Sprintf("max buffer (%d) length exceeded", h.usbBlock()), usbErrorFail)
	}

	ctx := h.initTransfer(transferIncoming)

	ctx.cmdBuf.WriteByte(cmdDebug)
	ctx.cmdBuf.WriteByte(debugReadMem8Bit)

	ctx.cmdBuf.WriteUint32LE(addr)
	ctx.cmdBuf.WriteUint16LE(len)

	// we need to fix read length for single bytes
	if readLen == 1 {
		readLen++
	}

	err := h.usbTransferNoErrCheck(ctx, readLen)

	if err != nil {
		return newUsbError(fmt.Sprintf("ReadMem8 transfer error occurred"), usbErrorFail)

	}

	buffer.Write(ctx.DataBytes())

	return h.usbGetReadWriteStatus()
}

/** */
func (h *StLink) usbReadMem16(addr uint32, len uint16, buffer *bytes.Buffer) error {
	if !h.version.flags.Get(flagHasMem16Bit) {
		return newUsbError("Read16 command not supported by device", usbErrorCommandNotFound)
	}

	/* data must be a multiple of 2 and half-word aligned */
	if ((len % 2) > 0) || ((addr % 2) > 0) {
		return newUsbError("ReadMem16 Invalid data alignment", usbErrorTargetUnalignedAccess)
	}

	ctx := h.initTransfer(transferIncoming)

	ctx.cmdBuf.WriteByte(cmdDebug)
	ctx.cmdBuf.WriteByte(debugApiV2ReadMem16Bit)

	ctx.cmdBuf.WriteUint32LE(addr)
	ctx.cmdBuf.WriteUint16LE(len)

	err := h.usbTransferNoErrCheck(ctx, uint32(len))

	if err != nil {
		return newUsbError("ReadMem16 transfer error occurred", usbErrorFail)
	}

	buffer.Write(ctx.DataBytes())

	return h.usbGetReadWriteStatus()
}

func (h *StLink) usbReadMem32(addr uint32, len uint16, buffer *bytes.Buffer) error {

	/* data must be a multiple of 4 and word aligned */
	if ((len % 4) > 0) || ((addr % 4) > 0) {
		return newUsbError("ReadMem32 Invalid data alignment", usbErrorTargetUnalignedAccess)
	}

	ctx := h.initTransfer(transferIncoming)

	ctx.cmdBuf.WriteByte(cmdDebug)
	ctx.cmdBuf.WriteByte(debugReadMem32Bit)

	ctx.cmdBuf.WriteUint32LE(addr)
	ctx.cmdBuf.WriteUint16LE(len)

	err := h.usbTransferNoErrCheck(ctx, uint32(len))

	if err != nil {
		return newUsbError("ReadMem32 transfer error occurred", usbErrorFail)
	}

	buffer.Write(ctx.DataBytes())

	return h.usbGetReadWriteStatus()
}

func (h *StLink) usbWriteMem8(addr uint32, len uint16, buffer []byte) error {
	writeLen := uint32(len)

	if writeLen > h.usbBlock() {
		return newUsbError(fmt.Sprintf("max buffer (%d) length exceeded", h.usbBlock()), usbErrorFail)
	}

	ctx := h.initTransfer(transferOutgoing)

	ctx.cmdBuf.WriteByte(cmdDebug)
	ctx.cmdBuf.WriteByte(debugWriteMem8Bit)

	ctx.cmdBuf.WriteUint32LE(addr)
	ctx.cmdBuf.WriteUint16LE(len)

	ctx.dataBuf.Write(buffer[:len])

	err := h.usbTransferNoErrCheck(ctx, writeLen)

	if err != nil {
		return err
	}

	return h.usbGetReadWriteStatus()
}

func (h *StLink) usbWriteMem16(addr uint32, len uint16, buffer []byte) error {
	writeLen := uint32(len)

	if !h.version.flags.Get(flagHasMem16Bit) {
		return newUsbError("Read16 command not supported by device", usbErrorCommandNotFound)
	}

	/* data must be a multiple of 2 and half-word aligned */
	if ((len % 2) > 0) || ((addr % 2) > 0) {
		return newUsbError("ReadMem16 Invalid data alignment", usbErrorTargetUnalignedAccess)
	}

	ctx := h.initTransfer(transferOutgoing)

	ctx.cmdBuf.WriteByte(cmdDebug)
	ctx.cmdBuf.WriteByte(debugApiV2WriteMem16Bit)

	ctx.cmdBuf.WriteUint32LE(addr)
	ctx.cmdBuf.WriteUint16LE(len)

	ctx.dataBuf.Write(buffer[:len])

	err := h.usbTransferNoErrCheck(ctx, writeLen)

	if err != nil {
		return err
	}

	return h.usbGetReadWriteStatus()
}

func (h *StLink) usbWriteMem32(addr uint32, len uint16, buffer []byte) error {
	writeLen := uint32(len)

	/* data must be a multiple of 4 and word aligned */
	if ((len % 4) > 0) || ((addr % 4) > 0) {
		return newUsbError("ReadMem32 Invalid data alignment", usbErrorTargetUnalignedAccess)
	}

	ctx := h.initTransfer(transferOutgoing)

	ctx.cmdBuf.WriteByte(cmdDebug)
	ctx.cmdBuf.WriteByte(debugWriteMem32Bit)

	ctx.cmdBuf.WriteUint32LE(addr)
	ctx.cmdBuf.WriteUint16LE(len)

	ctx.dataBuf.Write(buffer[:len])

	err := h.usbTransferNoErrCheck(ctx, writeLen)

	if err != nil {
		return err
	}

	return h.usbGetReadWriteStatus()
}
