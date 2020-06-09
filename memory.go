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

func (h *StLinkHandle) usbReadMem8(addr uint32, len uint16, buffer []byte) int {
	var readLen = uint32(len)

	/* max 8 bit read/write is 64 bytes or 512 bytes for v3 */
	if readLen > h.usb_block() {
		log.Debugf("max buffer (%d) length exceeded", h.usb_block())
		return ERROR_FAIL
	}

	h.usb_init_buffer(h.rx_ep, readLen)

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

	err := h.usb_xfer_noerrcheck(h.databuf, readLen)

	if err != nil {
		log.Error("Read8 xfer error ocurred")
		return ERROR_FAIL
	}

	copy(buffer, h.databuf)

	return h.usb_get_rw_status()
}

/** */
func (h *StLinkHandle) usbReadMem16(addr uint32, len uint16, buffer []byte) int {
	if (h.version.flags & STLINK_F_HAS_MEM_16BIT) == 0 {
		log.Debug("Read16 command not supported by device")
		return ERROR_COMMAND_NOTFOUND
	}

	/* data must be a multiple of 2 and half-word aligned */
	if ((len % 2) > 0) || ((addr % 2) > 0) {
		log.Debug("Read 16 Invalid data alignment")
		return ERROR_TARGET_UNALIGNED_ACCESS
	}

	h.usb_init_buffer(h.rx_ep, uint32(len))

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_READMEM_16BIT
	h.cmdidx++

	uint32ToLittleEndian(h.cmdbuf[h.cmdidx:], addr)
	h.cmdidx += 4

	uint16ToLittleEndian(h.cmdbuf[h.cmdidx:], len)
	h.cmdidx += 2

	err := h.usb_xfer_noerrcheck(h.databuf, uint32(len))

	if err != nil {
		log.Error("Read16 xfer error ocurred")
		return ERROR_FAIL
	}

	copy(buffer, h.databuf)

	return h.usb_get_rw_status()
}

func (h *StLinkHandle) usbReadMem32(addr uint32, len uint16, buffer []byte) int {

	/* data must be a multiple of 4 and word aligned */
	if ((len % 4) > 0) || ((addr % 4) > 0) {
		return ERROR_TARGET_UNALIGNED_ACCESS
	}

	h.usb_init_buffer(h.rx_ep, uint32(len))

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_READMEM_32BIT
	h.cmdidx++

	uint32ToLittleEndian(h.cmdbuf[h.cmdidx:], addr)
	h.cmdidx += 4

	uint16ToLittleEndian(h.cmdbuf[h.cmdidx:], len)
	h.cmdidx += 2

	err := h.usb_xfer_noerrcheck(h.databuf, uint32(len))

	if err != nil {
		log.Error("Read32 xfer error ocurred")
		return ERROR_FAIL
	}

	copy(buffer, h.databuf)

	return h.usb_get_rw_status()
}

func (h *StLinkHandle) usbReadMem(addr uint32, size uint32, count uint32, buffer []byte) int {
	var retval int = ERROR_OK
	var bytesRemaining uint32 = 0
	var retries int = 0
	var bufferPos uint32 = 0

	/* calculate byte count */
	count *= size

	/* switch to 8 bit if stlink does not support 16 bit memory read */
	if size == 2 && ((h.version.flags & STLINK_F_HAS_MEM_16BIT) == 0) {
		size = 1
	}

	for count > 0 {

		if size != 1 {
			bytesRemaining = h.max_block_size(h.max_mem_packet, addr)
		} else {
			bytesRemaining = h.usb_block()
		}

		if count < bytesRemaining {
			bytesRemaining = count
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
				var headBytes = size - (addr & (size - 1))

				retval = h.usbReadMem8(addr, uint16(headBytes), buffer)

				if retval == ERROR_WAIT && retries < MAX_WAIT_RETRIES {
					var sleepDur time.Duration = 1 << retries
					retries++

					time.Sleep(sleepDur * 1000000)
					continue
				}

				if retval != ERROR_OK {
					return retval
				}

				bufferPos += headBytes
				addr += headBytes
				count -= headBytes
				bytesRemaining -= headBytes
			}

			if (bytesRemaining & (size - 1)) > 0 {
				retval = h.usbReadMem(addr, 1, bytesRemaining, buffer[:bufferPos])
			} else if size == 2 {
				retval = h.usbReadMem16(addr, uint16(bytesRemaining), buffer[:bufferPos])
			} else {
				retval = h.usbReadMem32(addr, uint16(bytesRemaining), buffer[:bufferPos])
			}
		} else {
			retval = h.usbReadMem8(addr, uint16(bytesRemaining), buffer[:bufferPos])
		}

		if retval == ERROR_WAIT && retries < MAX_WAIT_RETRIES {
			var sleepDur time.Duration = 1 << retries
			retries++

			time.Sleep(sleepDur * 1000000)
			continue
		}

		if retval != ERROR_OK {
			return retval
		}

		bufferPos += bytesRemaining
		addr += bytesRemaining
		count -= bytesRemaining
	}

	return retval
}
