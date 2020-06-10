// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

// this code is mainly inspired and based on the openocd project source code
// for detailed information see

// https://sourceforge.net/p/openocd/code

package gostlink

import (
	"fmt"
)

func (h *StLinkHandle) usbReadMem8(addr uint32, len uint16, buffer []byte) error {
	var readLen = uint32(len)

	/* max 8 bit read/write is 64 bytes or 512 bytes for v3 */
	if readLen > h.usbBlock() {
		return NewUsbError(fmt.Sprintf("max buffer (%d) length exceeded", h.usbBlock()), ErrorFail)
	}

	h.usbInitBuffer(h.rx_ep, readLen)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_READMEM_8BIT
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
		return NewUsbError(fmt.Sprintf("ReadMem8 transfer error occurred"), ErrorFail)

	}

	copy(buffer, h.databuf)

	return h.usbGetReadWriteStatus()
}

/** */
func (h *StLinkHandle) usbReadMem16(addr uint32, len uint16, buffer []byte) error {
	if (h.version.flags & STLINK_F_HAS_MEM_16BIT) == 0 {
		return NewUsbError("Read16 command not supported by device", ErrorCommandNotFound)
	}

	/* data must be a multiple of 2 and half-word aligned */
	if ((len % 2) > 0) || ((addr % 2) > 0) {
		NewUsbError("ReadMem16 Invalid data alignment", ErrorTargetUnalignedAccess)
	}

	h.usbInitBuffer(h.rx_ep, uint32(len))

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_READMEM_16BIT
	h.cmdidx++

	uint32ToLittleEndian(h.cmdbuf[h.cmdidx:], addr)
	h.cmdidx += 4

	uint16ToLittleEndian(h.cmdbuf[h.cmdidx:], len)
	h.cmdidx += 2

	err := h.usbTransferNoErrCheck(h.databuf, uint32(len))

	if err != nil {
		return NewUsbError("ReadMem16 transfer error occurred", ErrorFail)
	}

	copy(buffer, h.databuf)

	return h.usbGetReadWriteStatus()
}

func (h *StLinkHandle) usbReadMem32(addr uint32, len uint16, buffer []byte) error {

	/* data must be a multiple of 4 and word aligned */
	if ((len % 4) > 0) || ((addr % 4) > 0) {
		return NewUsbError("ReadMem32 Invalid data alignment", ErrorTargetUnalignedAccess)
	}

	h.usbInitBuffer(h.rx_ep, uint32(len))

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_READMEM_32BIT
	h.cmdidx++

	uint32ToLittleEndian(h.cmdbuf[h.cmdidx:], addr)
	h.cmdidx += 4

	uint16ToLittleEndian(h.cmdbuf[h.cmdidx:], len)
	h.cmdidx += 2

	err := h.usbTransferNoErrCheck(h.databuf, uint32(len))

	if err != nil {
		return NewUsbError("ReadMem32 transfer error occurred", ErrorFail)
	}

	copy(buffer, h.databuf)

	return h.usbGetReadWriteStatus()
}
