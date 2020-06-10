// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

// this code is mainly inspired and based on the openocd project source code
// for detailed information see

// https://sourceforge.net/p/openocd/code

package gostlink

import (
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
)

func (h *StLinkHandle) usbInitBuffer(direction byte, size uint32) {
	h.direction = direction
	h.cmdidx = 0

	memset(h.cmdbuf, STLINK_SG_SIZE, 0)
	memset(h.databuf, STLINK_DATA_SIZE, 0)

	if h.version.stlink == 1 {
		h.usbTransferV1CreateCmd(direction, size)
	}
}

func (h *StLinkHandle) usbTransferNoErrCheck(buffer []byte, size uint32) error {
	var cmdSize int = STLINK_CMD_SIZE_V2

	if h.version.stlink == 1 {
		cmdSize = STLINK_SG_SIZE
		h.cmdbuf[14] = h.cmdidx - 15
	}

	err := h.usbTransferReadWrite(cmdSize, buffer, size)

	if err != nil {
		return err
	}

	if h.version.stlink == 1 {
		err := h.usbTransferV1GetStatus()

		if err == nil {
			if h.cmdbuf[12] == 1 {
				log.Debug("Check sense")

				err = h.usbTransferV1GetSense()
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (h *StLinkHandle) usbTransferErrCheck(buffer []byte, size uint32) error {

	err := h.usbTransferNoErrCheck(buffer, size)

	if err != nil {
		return err
	}

	return h.usbErrorCheck()
}

func (h *StLinkHandle) usbTransferReadWrite(cmdSize int, buffer []byte, size uint32) error {
	// write command buffer to tx_ep
	outP, err := h.usb_interface.OutEndpoint(int(h.tx_ep))

	if err != nil {
		return errors.New(fmt.Sprintf("could not open out endpoint #%d", int(h.tx_ep)))
	}

	_, err = usbWrite(outP, h.cmdbuf[:cmdSize])

	if err != nil {
		return err
	}

	if h.direction == h.tx_ep && size > 0 {
		_, err = usbWrite(outP, buffer[:size])

		if err != nil {
			return err
		}

	} else if h.direction == h.rx_ep && size > 0 {

		inP, err := h.usb_interface.InEndpoint(int(h.rx_ep))

		if err != nil {
			return errors.New(fmt.Sprintf("could not open in endpoint #%d", int(h.rx_ep)))
		}

		_, err = usbRead(inP, buffer)

		if err != nil {
			return err
		}
	}

	return nil
}

func (h *StLinkHandle) usbTransferV1CreateCmd(direction uint8, size uint32) {
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

func (h *StLinkHandle) usbTransferV1GetStatus() error {
	memset(h.cmdbuf, STLINK_SG_SIZE, 0)

	inEndpoint, err := h.usb_interface.InEndpoint(int(h.rx_ep))

	if err != nil {
		return err
	}

	bytesRead, err := usbRead(inEndpoint, h.cmdbuf)

	if err != nil || bytesRead != 13 {
		return errors.New("ST-Link V1 status read error")
	}

	t1 := buf_get_u32(h.cmdbuf, 0, 32)

	/* check for USBS */
	if t1 != 0x53425355 {
		return errors.New("ST-Link USBS check error")
	}

	/*
	 * CSW status:
	 * 0 success
	 * 1 command failure
	 * 2 phase error
	 */
	if h.cmdbuf[12] != 0 {
		return errors.New(fmt.Sprintf("got CSW status error %d", h.cmdbuf[12]))
	}

	return nil
}

func (h *StLinkHandle) usbTransferV1GetSense() error {

	h.usbInitBuffer(h.rx_ep, 16)

	h.cmdbuf[h.cmdidx] = REQUEST_SENSE
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = REQUEST_SENSE_LENGTH

	err := h.usbTransferReadWrite(REQUEST_SENSE_LENGTH, h.databuf, 16)

	if err != nil {
		return err
	} else {
		return h.usbTransferV1GetStatus()
	}
}

func (h *StLinkHandle) usbGetReadWriteStatus() error {

	if h.version.jtag_api == STLINK_JTAG_API_V1 {
		return nil
	}

	h.usbInitBuffer(h.rx_ep, 2)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++

	if (h.version.flags & STLINK_F_HAS_GETLASTRWSTATUS2) != 0 {
		h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_GETLASTRWSTATUS2
		h.cmdidx++

		return h.usbTransferErrCheck(h.databuf, 12)
	} else {
		h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_GETLASTRWSTATUS
		h.cmdidx++

		return h.usbTransferErrCheck(h.databuf, 2)
	}
}
