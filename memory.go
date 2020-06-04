// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

// this code is mainly inspired and based on the openocd project source code
// for detailed information see

// https://sourceforge.net/p/openocd/code

package gostlink

import (
	"time"

	log "github.com/sirupsen/logrus"
)

func (h *StLinkHandle) usb_read_mem8(addr uint32, len uint32, buffer []byte) int {
	var read_len = len

	/* max 8 bit read/write is 64 bytes or 512 bytes for v3 */
	if len > h.usb_block() {
		log.Debugf("max buffer (%d) length exceeded", h.usb_block())
		return ERROR_FAIL
	}

	h.usb_init_buffer(h.rx_ep, read_len)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_READMEM_8BIT
	h.cmdidx++

	h_u32_to_le(h.cmdbuf[h.cmdidx:], int(addr))
	h.cmdidx += 4

	h_u16_to_le(h.cmdbuf[h.cmdidx:], int(len))
	h.cmdidx += 2

	// we need to fix read length for single bytes
	if read_len == 1 {
		read_len++
	}

	err := h.usb_xfer_noerrcheck(h.databuf, read_len)

	if err != nil {
		log.Error("Read8 xfer error ocurred")
		return ERROR_FAIL
	}

	copy(buffer, h.databuf)

	return h.usb_get_rw_status()
}

/** */
func (h *StLinkHandle) usb_read_mem16(addr uint32, len uint32, buffer []byte) int {
	if (h.version.flags & STLINK_F_HAS_MEM_16BIT) == 0 {
		log.Debug("Read16 command not supported by device")
		return ERROR_COMMAND_NOTFOUND
	}

	/* data must be a multiple of 2 and half-word aligned */
	if ((len % 2) > 0) || ((addr % 2) > 0) {
		log.Debug("Read 16 Invalid data alignment")
		return ERROR_TARGET_UNALIGNED_ACCESS
	}

	h.usb_init_buffer(h.rx_ep, len)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_READMEM_16BIT
	h.cmdidx++

	h_u32_to_le(h.cmdbuf[h.cmdidx:], int(addr))
	h.cmdidx += 4

	h_u16_to_le(h.cmdbuf[h.cmdidx:], int(len))
	h.cmdidx += 2

	err := h.usb_xfer_noerrcheck(h.databuf, len)

	if err != nil {
		log.Error("Read16 xfer error ocurred")
		return ERROR_FAIL
	}

	copy(buffer, h.databuf)

	return h.usb_get_rw_status()
}

func (h *StLinkHandle) usb_read_mem32(addr uint32, len uint32, buffer []byte) int {

	/* data must be a multiple of 4 and word aligned */
	if ((len % 4) > 0) || ((addr % 4) > 0) {
		return ERROR_TARGET_UNALIGNED_ACCESS
	}

	h.usb_init_buffer(h.rx_ep, len)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_READMEM_32BIT
	h.cmdidx++

	h_u32_to_le(h.cmdbuf[h.cmdidx:], int(addr))
	h.cmdidx += 4

	h_u16_to_le(h.cmdbuf[h.cmdidx:], int(len))
	h.cmdidx += 2

	err := h.usb_xfer_noerrcheck(h.databuf, len)

	if err != nil {
		log.Error("Read32 xfer error ocurred")
		return ERROR_FAIL
	}

	copy(buffer, h.databuf)

	return h.usb_get_rw_status()
}

func (h *StLinkHandle) usb_read_mem(addr uint32, size uint32, count uint32, buffer []byte) int {
	var retval int = ERROR_OK
	var bytes_remaining uint32 = 0
	var retries int = 0
	var buffer_pos uint32 = 0

	/* calculate byte count */
	count *= size

	/* switch to 8 bit if stlink does not support 16 bit memory read */
	if size == 2 && ((h.version.flags & STLINK_F_HAS_MEM_16BIT) == 0) {
		size = 1
	}

	for count > 0 {

		if size != 1 {
			bytes_remaining = h.max_block_size(h.max_mem_packet, addr)
		} else {
			bytes_remaining = h.usb_block()
		}

		if count < uint32(bytes_remaining) {
			bytes_remaining = count
		}

		/*
		* all stlink support 8/32bit memory read/writes and only from
		* stlink V2J26 there is support for 16 bit memory read/write.
		* Honour 32 bit and, if possible, 16 bit too. Otherwise, handle
		* as 8bit access.
		 */
		if size != 1 {
			/* When in jtag mode the stlink uses the auto-increment functionality.
			 	* However it expects us to pass the data correctly, this includes
			 	* alignment and any page boundaries. We already do this as part of the
			 	* adi_v5 implementation, but the stlink is a hla adapter and so this
			 	* needs implementing manually.
				 * currently this only affects jtag mode, according to ST they do single
				 * access in SWD mode - but this may change and so we do it for both modes */

			/* we first need to check for any unaligned bytes */
			if (addr & (size - 1)) > 0 {
				var head_bytes = size - (addr & (size - 1))

				retval = h.usb_read_mem8(addr, head_bytes, buffer)

				if retval == ERROR_WAIT && retries < MAX_WAIT_RETRIES {
					var sleep_dur time.Duration = (1 << retries)
					retries++

					time.Sleep(sleep_dur * 1000000)
					continue
				}

				if retval != ERROR_OK {
					return retval
				}

				buffer_pos += head_bytes
				addr += head_bytes
				count -= head_bytes
				bytes_remaining -= head_bytes
			}

			if (uint32(bytes_remaining) & (size - 1)) > 0 {
				retval = h.usb_read_mem(addr, 1, bytes_remaining, buffer[:buffer_pos])
			} else if size == 2 {
				retval = h.usb_read_mem16(addr, bytes_remaining, buffer[:buffer_pos])
			} else {
				retval = h.usb_read_mem32(addr, bytes_remaining, buffer[:buffer_pos])
			}
		} else {
			retval = h.usb_read_mem8(addr, bytes_remaining, buffer[:buffer_pos])
		}

		if retval == ERROR_WAIT && retries < MAX_WAIT_RETRIES {
			var sleep_dur time.Duration = (1 << retries)
			retries++

			time.Sleep(sleep_dur * 1000000)
			continue
		}

		if retval != ERROR_OK {
			return retval
		}

		buffer_pos += bytes_remaining
		addr += bytes_remaining
		count -= bytes_remaining

	}

	return retval
}
