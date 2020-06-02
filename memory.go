// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

// this code is mainly inspired and based on the openocd project source code
// for detailed information see

// https://sourceforge.net/p/openocd/code

package gostlink

import (
	"bytes"
	"fmt"
)

func (h *StLinkHandle) usbReadMem8(addr uint32, len uint16, buffer *bytes.Buffer) error {
	var readLen = uint32(len)

	/* max 8 bit read/write is 64 bytes or 512 bytes for v3 */
	if readLen > h.usbBlock() {
		return newUsbError(fmt.Sprintf("max buffer (%d) length exceeded", h.usbBlock()), usbErrorFail)
	}

	h.usbInitBuffer(transferRxEndpoint, readLen)

	h.cmdbuf[h.cmdidx] = cmdDebug
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = debugReadMem8Bit
	h.cmdidx++

	uint32ToLittleEndian(h.cmdbuf[h.cmdidx:], addr)
	h.cmdidx += 4

	uint16ToLittleEndian(h.cmdbuf[h.cmdidx:], len)
	h.cmdidx += 2

	// we need to fix read length for single bytes
	if readLen == 1 {
		readLen++
	}

	err := h.usbTransferNoErrCheck(h.databuf, readLen)

	if err != nil {
		return newUsbError(fmt.Sprintf("ReadMem8 transfer error occurred"), usbErrorFail)

	}

	buffer.Write(h.databuf[:len])

	return h.usbGetReadWriteStatus()
}

/** */
func (h *StLinkHandle) usbReadMem16(addr uint32, len uint16, buffer *bytes.Buffer) error {
	if (h.version.flags & flagHasMem16Bit) == 0 {
		return newUsbError("Read16 command not supported by device", usbErrorCommandNotFound)
	}

	/* data must be a multiple of 2 and half-word aligned */
	if ((len % 2) > 0) || ((addr % 2) > 0) {
		return newUsbError("ReadMem16 Invalid data alignment", usbErrorTargetUnalignedAccess)
	}

	h.usbInitBuffer(transferRxEndpoint, uint32(len))

	h.cmdbuf[h.cmdidx] = cmdDebug
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = debugApiV2ReadMem16Bit
	h.cmdidx++

	uint32ToLittleEndian(h.cmdbuf[h.cmdidx:], addr)
	h.cmdidx += 4

	uint16ToLittleEndian(h.cmdbuf[h.cmdidx:], len)
	h.cmdidx += 2

	err := h.usbTransferNoErrCheck(h.databuf, uint32(len))

	if err != nil {
		return newUsbError("ReadMem16 transfer error occurred", usbErrorFail)
	}

	buffer.Write(h.databuf[:len])

	return h.usbGetReadWriteStatus()
}

func (h *StLinkHandle) usbReadMem32(addr uint32, len uint16, buffer *bytes.Buffer) error {

	/* data must be a multiple of 4 and word aligned */
	if ((len % 4) > 0) || ((addr % 4) > 0) {
		return newUsbError("ReadMem32 Invalid data alignment", usbErrorTargetUnalignedAccess)
	}

	h.usbInitBuffer(transferRxEndpoint, uint32(len))

	h.cmdbuf[h.cmdidx] = cmdDebug
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = debugReadMem32Bit
	h.cmdidx++

	uint32ToLittleEndian(h.cmdbuf[h.cmdidx:], addr)
	h.cmdidx += 4

	uint16ToLittleEndian(h.cmdbuf[h.cmdidx:], len)
	h.cmdidx += 2

	err := h.usbTransferNoErrCheck(h.databuf, uint32(len))

	if err != nil {
		return newUsbError("ReadMem32 transfer error occurred", usbErrorFail)
	}

	buffer.Write(h.databuf[:len])

	return h.usbGetReadWriteStatus()
}

func (h *StLinkHandle) usbWriteMem8(address uint32, len uint16, buffer []byte) error {
	writeLen := uint32(len)

	if writeLen > h.usbBlock() {
		return newUsbError(fmt.Sprintf("max buffer (%d) length exceeded", h.usbBlock()), usbErrorFail)
	}

	h.usbInitBuffer(transferTxEndpoint, writeLen)

	h.cmdbuf[h.cmdidx] = cmdDebug
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = debugWriteMem8Bit
	h.cmdidx++

	uint32ToLittleEndian(h.cmdbuf[h.cmdidx:], address)
	h.cmdidx += 4

	uint16ToLittleEndian(h.cmdbuf[h.cmdidx:], len)
	h.cmdidx += 2

	err := h.usbTransferNoErrCheck(buffer, writeLen)

	if err != nil {
		return err
	}

	return h.usbGetReadWriteStatus()
}

func (h *StLinkHandle) usbWriteMem16(address uint32, len uint16, buffer []byte) error {
	writeLen := uint32(len)

	if (h.version.flags & flagHasMem16Bit) == 0 {
		return newUsbError("Read16 command not supported by device", usbErrorCommandNotFound)
	}

	/* data must be a multiple of 2 and half-word aligned */
	if ((len % 2) > 0) || ((address % 2) > 0) {
		return newUsbError("ReadMem16 Invalid data alignment", usbErrorTargetUnalignedAccess)
	}

	h.usbInitBuffer(transferTxEndpoint, writeLen)

	h.cmdbuf[h.cmdidx] = cmdDebug
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = debugApiV2WriteMem16Bit
	h.cmdidx++

	uint32ToLittleEndian(h.cmdbuf[h.cmdidx:], address)
	h.cmdidx += 4

	uint16ToLittleEndian(h.cmdbuf[h.cmdidx:], len)
	h.cmdidx += 2

	err := h.usbTransferNoErrCheck(buffer, writeLen)

	if err != nil {
		return err
	}

	return h.usbGetReadWriteStatus()
}

func (h *StLinkHandle) usbWriteMem32(address uint32, len uint16, buffer []byte) error {
	writeLen := uint32(len)

	/* data must be a multiple of 4 and word aligned */
	if ((len % 4) > 0) || ((address % 4) > 0) {
		return newUsbError("ReadMem32 Invalid data alignment", usbErrorTargetUnalignedAccess)
	}

	h.usbInitBuffer(transferTxEndpoint, writeLen)

	h.cmdbuf[h.cmdidx] = cmdDebug
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = debugWriteMem32Bit
	h.cmdidx++

	uint32ToLittleEndian(h.cmdbuf[h.cmdidx:], address)
	h.cmdidx += 4

	uint16ToLittleEndian(h.cmdbuf[h.cmdidx:], len)
	h.cmdidx += 2

	err := h.usbTransferNoErrCheck(buffer, writeLen)

	if err != nil {
		return err
	}

	return h.usbGetReadWriteStatus()
}
