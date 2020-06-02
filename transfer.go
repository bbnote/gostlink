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
	"time"

	log "github.com/sirupsen/logrus"
)

func (h *StLinkHandle) usbInitBuffer(endpoint usbTransferEndpoint, size uint32) {
	h.transferEndpoint = endpoint
	h.cmdidx = 0

	memset(h.cmdbuf, cmdBufferSize, 0)
	memset(h.databuf, dataBufferSize, 0)

	if h.version.stlink == 1 {
		h.usbTransferV1CreateCmd(endpoint, size)
	}
}

func (h *StLinkHandle) usbTransferNoErrCheck(buffer []byte, size uint32) error {
	var cmdSize int = cmdSizeV2

	if h.version.stlink == 1 {
		cmdSize = cmdBufferSize
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

	_, err := usbWrite(h.txEndpoint, h.cmdbuf[:cmdSize])

	if err != nil {
		return err
	}

	if h.transferEndpoint == transferTxEndpoint && size > 0 {
		time.Sleep(time.Millisecond * 10)

		_, err = usbWrite(h.txEndpoint, buffer[:size])

		if err != nil {
			return err
		}

	} else if h.transferEndpoint == transferRxEndpoint && size > 0 {

		_, err = usbRead(h.rxEndpoint, buffer[:size])

		if err != nil {
			return err
		}
	}

	return nil
}

func (h *StLinkHandle) usbTransferV1CreateCmd(endpoint usbTransferEndpoint, size uint32) {
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
	if endpoint == transferRxEndpoint {
		h.cmdbuf[h.cmdidx] = usbEndpointIn
	} else {
		h.cmdbuf[h.cmdidx] = usbEndpointOut
	}
	h.cmdidx++

	h.cmdbuf[h.cmdidx] = 0 /* lun */
	h.cmdidx++

	/* cdb clength (is filled in at xfer) */
	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++
}

func (h *StLinkHandle) usbTransferV1GetStatus() error {
	memset(h.cmdbuf, cmdBufferSize, 0)

	bytesRead, err := usbRead(h.rxEndpoint, h.cmdbuf)

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

	h.usbInitBuffer(transferRxEndpoint, 16)

	h.cmdbuf[h.cmdidx] = cmdRequestSense
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = 0
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = requestSenseLength

	err := h.usbTransferReadWrite(requestSenseLength, h.databuf, 16)

	if err != nil {
		return err
	} else {
		return h.usbTransferV1GetStatus()
	}
}

func (h *StLinkHandle) usbGetReadWriteStatus() error {

	if h.version.jtagApi == jTagApiV1 {
		return nil
	}

	h.usbInitBuffer(transferRxEndpoint, 2)

	h.cmdbuf[h.cmdidx] = cmdDebug
	h.cmdidx++

	if (h.version.flags & flagHasGetLastRwStatus2) != 0 {
		h.cmdbuf[h.cmdidx] = debugApiV2GetLastRWStatus2
		h.cmdidx++

		return h.usbTransferErrCheck(h.databuf, 12)
	} else {
		h.cmdbuf[h.cmdidx] = debugApiV2GetLastRWStatus
		h.cmdidx++

		return h.usbTransferErrCheck(h.databuf, 2)
	}
}
