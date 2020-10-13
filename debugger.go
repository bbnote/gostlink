// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"errors"
	"time"
)

/** Issue an STLINK command via USB transfer, with retries on any wait status responses.

  Works for commands where the STLINK_DEBUG status is returned in the first
  byte of the response packet. For SWIM a SWIM_READSTATUS is requested instead.

  Returns an openocd result code.
*/
func (h *StLink) usbCmdAllowRetry(ctx *transferCtx, size uint32) error {
	var retries int = 0

	for true {
		if (h.stMode != StLinkModeDebugSwim) || retries > 0 {
			err := h.usbTransferNoErrCheck(ctx, size)
			if err != nil {
				return err
			}
		}

		/*
			    TODO: Implement DEBUG swim!
			if (h.st_mode == STLINK_MODE_DEBUG_SWIM) {
				err = h.stlink_swim_status(handle);
				if err != nil {
					return err
				}
			}*/

		err := h.usbErrorCheck(ctx)

		if err != nil {
			usbError := err.(*usbError)

			if usbError.UsbErrorCode == usbErrorWait && retries < maximumWaitRetries {
				var delayUs time.Duration = (1 << retries) * 1000

				retries++
				logger.Debugf("cmdAllowRetry ERROR_WAIT, retry %d, delaying %d microseconds", retries, delayUs)
				time.Sleep(delayUs * 1000)

				continue
			}
		}

		return err
	}

	return errors.New("invalid cmd allow retry state")
}

func (h *StLink) usbAssertSrst(srst byte) error {

	/* TODO:
		* Implement SWIM debugger
	     *
		if h.st_mode == STLINK_MODE_DEBUG_SWIM {
			return stlink_swim_assert_reset(handle, srst);
		}
	*/

	if h.version.stlink == 1 {
		return errors.New("rsrt command not supported by st-link V1")
	}

	ctx := h.initTransfer(transferIncoming)

	ctx.cmdBuf.WriteByte(cmdDebug)
	ctx.cmdBuf.WriteByte(debugApiV2DriveNrst)
	ctx.cmdBuf.WriteByte(srst)

	return h.usbCmdAllowRetry(ctx, 2)
}
