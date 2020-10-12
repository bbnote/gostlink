// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/boljen/go-bitmap"
	"github.com/google/gousb"
)

var (
	libUsbCtx *gousb.Context = nil
)

func InitUsb() error {
	if libUsbCtx == nil {

		libUsbCtx = gousb.NewContext()
		libUsbCtx.Debug(3)

		if libUsbCtx != nil {
			return nil
		} else {
			return errors.New("could not initialize libusb context")
		}
	} else {
		logger.Warn("libusb context already initialized")
		return nil
	}
}

func CloseUSB() {
	if libUsbCtx != nil {
		libUsbCtx.Close()
	} else {
		logger.Warn("tried to close non initialized libusb context")
	}
}

func usbFindDevices(vids []gousb.ID, pids []gousb.ID) ([]*gousb.Device, error) {
	devices, err := libUsbCtx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		if idExists(vids, desc.Vendor) == true && idExists(pids, desc.Product) == true {
			logger.Debugf("inspect usb device [%04x:%04x] on bus %03d:%03d...", uint16(desc.Vendor), uint16(desc.Product), desc.Bus, desc.Address)

			return true
		} else {
			return false
		}
	})

	// Error of OpenDevices is ignored cause of lack
	// of information on which specific device the error
	// occurred. So as long we got a valid device handle
	// returned there is no actual error

	if len(devices) > 0 {
		return devices, nil
	} else {
		return nil, err
	}
}

func usbWrite(endpoint *gousb.OutEndpoint, buffer []byte) (int, error) {

	opCtx := context.Background()

	var done func()
	opCtx, done = context.WithTimeout(opCtx, time.Millisecond*10000)
	defer done()

	bytesWritten, err := endpoint.WriteContext(opCtx, buffer)

	if err != nil {
		return -1, err
	} else {
		logger.Tracef("%d Bytes -> EP-%d", bytesWritten, endpoint.Desc.Number)
		return bytesWritten, nil
	}

}

func usbRead(endpoint *gousb.InEndpoint, buffer []byte) (int, error) {
	opCtx := context.Background()

	var done func()
	opCtx, done = context.WithTimeout(opCtx, time.Millisecond*50)
	defer done()

	bytesRead, err := endpoint.ReadContext(opCtx, buffer)

	if err != nil {
		return -1, err
	} else {
		logger.Tracef("EP-%d -> %d Bytes", endpoint.Desc.Number, bytesRead)
		return bytesRead, nil
	}
}

func (h *StLink) usbGetVersion() error {
	var v, x, y, jtag, swim, msd, bridge byte = 0, 0, 0, 0, 0, 0, 0

	ctx := h.initTransfer(transferRxEndpoint)

	ctx.cmdBuffer.WriteByte(cmdGetVersion)

	err := h.usbTransferNoErrCheck(ctx, 6)

	if err != nil {
		return err
	}

	version := be_to_h_u16(ctx.dataBuffer.Bytes())

	v = byte((version >> 12) & 0x0f)
	x = byte((version >> 6) & 0x3f)
	y = byte(version & 0x3f)

	h.vid = gousb.ID(le_to_h_u16(ctx.dataBuffer.Bytes()[2:]))
	h.pid = gousb.ID(le_to_h_u16(ctx.dataBuffer.Bytes()[4:]))

	switch h.pid {
	case stLinkV21Pid, stLinkV21NoMsdPid:
		if (x <= 22 && y == 7) || (x >= 25 && y >= 7 && y <= 12) {
			msd = x
			swim = y
			jtag = 0
		} else {
			jtag = x
			msd = y
			swim = 0
		}

	default:
		jtag = x
		msd = 0
		swim = y
	}

	/* STLINK-V3 requires a specific command */
	if v == 3 && x == 0 && y == 0 {
		ctxV3 := h.initTransfer(transferRxEndpoint)

		ctxV3.cmdBuffer.WriteByte(debugApiV3GetVersionEx)

		err := h.usbTransferNoErrCheck(ctxV3, 12)

		if err != nil {
			return err
		}

		v = ctxV3.dataBuffer.Bytes()[0]
		swim = ctxV3.dataBuffer.Bytes()[1]
		jtag = ctxV3.dataBuffer.Bytes()[2]
		msd = ctxV3.dataBuffer.Bytes()[3]
		bridge = ctxV3.dataBuffer.Bytes()[4]
		h.vid = gousb.ID(le_to_h_u16(ctxV3.dataBuffer.Bytes()[8:]))
		h.pid = gousb.ID(le_to_h_u16(ctxV3.dataBuffer.Bytes()[10:]))
	}

	h.version.stlink = int(v)
	h.version.jtag = int(jtag)
	h.version.swim = int(swim)

	var flags bitmap.Bitmap = bitmap.New(32)

	switch h.version.stlink {
	case 1:
		/* ST-LINK/V1 from J11 switch to api-v2 (and support SWD) */
		if h.version.jtag >= 11 {
			h.version.jtagApi = jTagApiV2
		} else {
			h.version.jtagApi = jTagApiV1
		}
	case 2:
		/* all ST-LINK/V2 and ST-Link/V2.1 use api-v2 */
		h.version.jtagApi = jTagApiV2

		/* API for trace from J13 */
		/* API for target voltage from J13 */
		if h.version.jtag >= 13 {
			flags.Set(flagHasTrace, true)
		}

		/* preferred API to get last R/W status from J15 */
		if h.version.jtag >= 15 {
			flags.Set(flagHasGetLastRwStatus2, true)
		}

		/* API to set SWD frequency from J22 */
		if h.version.jtag >= 22 {
			flags.Set(flagHasSwdSetFreq, true)
		}

		/* API to set JTAG frequency from J24 */
		/* API to access DAP registers from J24 */
		if h.version.jtag >= 24 {
			flags.Set(flagHasJtagSetFreq, true)
			flags.Set(flagHasDapReg, true)
		}

		/* Quirk for read DP in JTAG mode (V2 only) from J24, fixed in J32 */
		if h.version.jtag >= 24 && h.version.jtag < 32 {
			flags.Set(flagQuirkJtagDpRead, true)
		}

		/* API to read/write memory at 16 bit from J26 */
		if h.version.jtag >= 26 {
			flags.Set(flagHasMem16Bit, true)
		}

		/* API required to init AP before any AP access from J28 */
		if h.version.jtag >= 28 {
			flags.Set(flagHasApInit, true)
		}

		/* API required to return proper error code on close AP from J29 */
		if h.version.jtag >= 29 {
			flags.Set(flagFixCloseAp, true)
		}

		/* Banked regs (DPv1 & DPv2) support from V2J32 */
		if h.version.jtag >= 32 {
			flags.Set(flagHasDpBankSel, true)
		}
	case 3:
		/* all STLINK-V3 use api-v3 */
		h.version.jtagApi = jTagApiV3

		/* STLINK-V3 is a superset of ST-LINK/V2 */
		flags.Set(flagHasTrace, true)            // API for trace and for target voltage
		flags.Set(flagHasGetLastRwStatus2, true) // preferred API to get last R/W status
		flags.Set(flagHasDapReg, true)           // API to access DAP registers
		flags.Set(flagHasMem16Bit, true)         // API to read/write memory at 16 bit
		flags.Set(flagHasApInit, true)           // API required to init AP before any AP access
		flags.Set(flagFixCloseAp, true)          // API required to return proper error code on close AP

		if h.version.jtag >= 2 {
			flags.Set(flagHasDpBankSel, true) // Banked regs (DPv1 & DPv2) support from V3J2
		}

		if h.version.jtag >= 6 {
			flags.Set(flagHasRw8Bytes512, true) // 8bit read/write max packet size 512 bytes from V3J6
		}

	default:
		break
	}

	h.version.flags = flags

	var vStr string = fmt.Sprintf("V%d", v)

	if jtag > 0 || msd != 0 {
		vStr += fmt.Sprintf("J%d", jtag)
	}

	if msd > 0 {
		vStr += fmt.Sprintf("M%d", msd)
	}

	if bridge > 0 {
		vStr += fmt.Sprintf("B%d", bridge)
	}

	serialNo, _ := h.libUsbDevice.SerialNumber()

	logger.Debugf("parsed st-link version [%s] for [%s]", vStr, serialNo)

	return nil
}

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

	ctx := h.initTransfer(transferRxEndpoint)

	ctx.cmdBuffer.WriteByte(cmdDebug)
	ctx.cmdBuffer.WriteByte(debugApiV2DriveNrst)
	ctx.cmdBuffer.WriteByte(srst)

	return h.usbCmdAllowRetry(ctx, 2)
}

func (h *StLink) maxBlockSize(tarAutoIncrBlock uint32, address uint32) uint32 {
	var maxTarBlock = tarAutoIncrBlock - ((tarAutoIncrBlock - 1) & address)

	if maxTarBlock == 0 {
		maxTarBlock = 4
	}

	return maxTarBlock
}

func (h *StLink) usbBlock() uint32 {
	if h.version.flags.Get(flagHasRw8Bytes512) {
		return v3MaxReadWrite8
	} else {
		return maxReadWrite8
	}
}
