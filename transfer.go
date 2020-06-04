// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

// this code is mainly inspired and based on the openocd project source code
// for detailed information see

// https://sourceforge.net/p/openocd/code

package gostlink

import (
	"errors"

	log "github.com/sirupsen/logrus"
)

func (h *StLinkHandle) usb_init_buffer(direction byte, size uint32) {
	h.direction = direction
	h.cmdidx = 0

	memset(h.cmdbuf, STLINK_SG_SIZE, 0)
	memset(h.databuf, STLINK_DATA_SIZE, 0)

	if h.version.stlink == 1 {
		h.usb_xfer_v1_create_cmd(direction, size)
	}
}

func (h *StLinkHandle) usb_xfer_noerrcheck(buffer []byte, size uint32) error {
	var cmdsize int = STLINK_CMD_SIZE_V2

	if h.version.stlink == 1 {
		cmdsize = STLINK_SG_SIZE
		h.cmdbuf[14] = h.cmdidx - 15
	}

	err := h.usb_xfer_rw(cmdsize, buffer, size)

	if err != nil {
		return err
	}

	if h.version.stlink == 1 {
		err := h.usb_xfer_v1_get_status()

		if err == nil {
			if h.cmdbuf[12] == 1 {
				log.Debug("Check sense")

				err = h.usb_xfer_v1_get_sense()
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (h *StLinkHandle) usb_xfer_errcheck(buffer []byte, size uint32) int {

	err := h.usb_xfer_noerrcheck(buffer, size)

	if err != nil {
		log.Error(err)
		return ERROR_FAIL
	}

	return h.usb_error_check()
}

func (h *StLinkHandle) usb_xfer_rw(cmdsize int, buffer []byte, size uint32) error {
	// write command buffer to tx_ep
	outP, err := h.usb_interface.OutEndpoint(int(h.tx_ep))

	if err != nil {
		return errors.New("Could not open out endpoint")
	}

	_, err = usb_write(outP, h.cmdbuf[:cmdsize])

	if err != nil {
		return err
	}

	if h.direction == h.tx_ep && size > 0 {
		_, err = usb_write(outP, buffer[:size])

		if err != nil {
			return err
		}

	} else if h.direction == h.rx_ep && size > 0 {

		inP, err := h.usb_interface.InEndpoint(int(h.rx_ep))

		if err != nil {
			return errors.New("Could not get in endpoint")
		}

		_, err = usb_read(inP, buffer)

		if err != nil {
			return err
		}
	}

	return nil
}

func (h *StLinkHandle) usb_xfer_v1_create_cmd(direction uint8, size uint32) {
	h.cmdbuf[0] = 'U'
	h.cmdbuf[1] = 'S'
	h.cmdbuf[2] = 'B'
	h.cmdbuf[3] = 'C'
	h.cmdidx += 4

	buf_set_u32(h.cmdbuf[:h.cmdidx], 0, 32, 0)
	h.cmdidx += 4

	buf_set_u32(h.cmdbuf[:h.cmdidx], 0, 32, size)
	h.cmdidx += 4

	/* cbw flags */
	if direction == h.rx_ep {
		h.cmdbuf[h.cmdidx] = ENDPOINT_IN
	} else {
		h.cmdbuf[h.cmdidx] = ENDPOINT_OUT
	}
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = 0 /* lun */
	h.cmdidx++

	/* cdb clength (is filled in at xfer) */
	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++
}

func (h *StLinkHandle) usb_xfer_v1_get_status() error {
	memset(h.cmdbuf, STLINK_SG_SIZE, 0)

	in_endpoint, err := h.usb_interface.InEndpoint(int(h.rx_ep))

	if err != nil {
		return err
	}

	var b_read int = 0

	b_read, err = usb_read(in_endpoint, h.cmdbuf)

	if err != nil || b_read != 13 {
		return errors.New("ST-Link V1 status read error")
	}

	t1 := buf_get_u32(h.cmdbuf, 0, 32)

	/* check for USBS */
	if t1 != 0x53425355 {
		return errors.New("No USBS")
	}

	/*
	 * CSW status:
	 * 0 success
	 * 1 command failure
	 * 2 phase error
	 */
	if h.cmdbuf[12] != 0 {
		log.Errorf("Got CSW status: %d", h.cmdbuf[12])
		return errors.New("GOT CSW status error")
	}

	return nil
}

func (h *StLinkHandle) usb_xfer_v1_get_sense() error {

	h.usb_init_buffer(h.rx_ep, 16)

	h.cmdbuf[h.cmdidx] = REQUEST_SENSE
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = REQUEST_SENSE_LENGTH

	err := h.usb_xfer_rw(REQUEST_SENSE_LENGTH, h.databuf, 16)

	if err != nil {
		return err
	} else {
		err := h.usb_xfer_v1_get_status()
		return err
	}
}

func (h *StLinkHandle) usb_get_rw_status() int {

	if h.version.jtag_api == STLINK_JTAG_API_V1 {
		return ERROR_OK
	}

	h.usb_init_buffer(h.rx_ep, 2)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++

	if (h.version.flags & STLINK_F_HAS_GETLASTRWSTATUS2) != 0 {
		h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_GETLASTRWSTATUS2
		h.cmdidx++

		return h.usb_xfer_errcheck(h.databuf, 12)
	} else {
		h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_GETLASTRWSTATUS
		h.cmdidx++

		return h.usb_xfer_errcheck(h.databuf, 2)
	}
}
