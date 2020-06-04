// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

// this code is mainly inspired and based on the openocd project source code
// for detailed information see

// https://sourceforge.net/p/openocd/code

package gostlink

import "errors"

func (h *StLinkHandle) usb_read_mem32(addr uint32, len uint16, buffer []byte) (int, error) {

	/* data must be a multiple of 4 and word aligned */
	if ((len % 4) > 0) || ((addr % 4) > 0) {
		return ERROR_TARGET_UNALIGNED_ACCESS, errors.New("Invalid data alignment")
	}

	h.usb_init_buffer(h.rx_ep, uint32(len))

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_READMEM_32BIT
	h.cmdidx++

	h_u32_to_le(h.cmdbuf[h.cmdidx:], int(addr))
	h.cmdidx += 4

	h_u16_to_le(h.cmdbuf[h.cmdidx:], int(len))
	h.cmdidx += 2

	err := h.usb_xfer_noerrcheck(h.databuf, int(len))

	if err != nil {
		return ERROR_FAIL, err
	}

	copy(buffer, h.databuf)

	return h.usb_get_rw_status(), nil
}
