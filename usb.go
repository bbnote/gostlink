// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"context"
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/google/gousb"
)

var usbCtx *gousb.Context = nil

func InitializeUSB() error {
	if usbCtx == nil {
		usbCtx = gousb.NewContext()
		usbCtx.Debug(0)

		if usbCtx != nil {
			log.Debug("Initialized libsusb...")
			return nil
		} else {
			return errors.New("Could not initialize libusb!")
		}
	} else {
		log.Warn("USB already initialized!")
		return nil
	}
}

func CloseUSB() {
	if usbCtx != nil {
		usbCtx.Close()
	} else {
		log.Warn("Could not close uninitialized usb context")
	}
}

func usbFindDevices(vids []gousb.ID, pids []gousb.ID) ([]*gousb.Device, error) {
	devices, err := usbCtx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		if idExists(vids, desc.Vendor) == true && idExists(pids, desc.Product) == true {
			log.Infof("Found USB device [%04x:%04x] on bus %03d:%03d", uint16(desc.Vendor), uint16(desc.Product), desc.Bus, desc.Address)

			return true
		} else {
			return false
		}
	})

	if len(devices) > 0 && err != nil {
		log.Warn("Found devices but an error occured during scan (", err, ")")
		return devices, nil
	} else if err == nil {
		log.Infof("Found %d matching devices based on vendor and product id list", len(devices))
		return devices, nil
	} else {
		log.Error("Got error during usb device scan", err)
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
		log.Tracef("Wrote %d bytes to endpoint", bytesWritten)
		return bytesWritten, nil
	}

}

type contextReader interface {
	ReadContext(context.Context, []byte) (int, error)
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
		log.Tracef("Read %d byte from in endpoint", bytesRead)
		return bytesRead, nil
	}
}

func (h *StLinkHandle) usbGetVersion() error {
	var v, x, y, jtag, swim, msd, bridge byte = 0, 0, 0, 0, 0, 0, 0

	h.usbInitBuffer(transferRxEndpoint, 6)

	h.cmdbuf[h.cmdidx] = cmdGetVersion
	h.cmdidx++

	err := h.usbTransferNoErrCheck(h.databuf, 6)

	if err != nil {
		return err
	}

	version := be_to_h_u16(h.databuf)

	v = byte((version >> 12) & 0x0f)
	x = byte((version >> 6) & 0x3f)
	y = byte(version & 0x3f)

	h.vid = gousb.ID(le_to_h_u16(h.databuf[2:]))
	h.pid = gousb.ID(le_to_h_u16(h.databuf[4:]))

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
		h.usbInitBuffer(transferRxEndpoint, 16)

		h.cmdbuf[h.cmdidx] = debugApiV3GetVersionEx
		h.cmdidx++

		err := h.usbTransferNoErrCheck(h.databuf, 12)

		if err != nil {
			return err
		}

		v = h.databuf[0]
		swim = h.databuf[1]
		jtag = h.databuf[2]
		msd = h.databuf[3]
		bridge = h.databuf[4]
		h.vid = gousb.ID(le_to_h_u16(h.databuf[8:]))
		h.pid = gousb.ID(le_to_h_u16(h.databuf[10:]))
	}

	h.version.stlink = int(v)
	h.version.jtag = int(jtag)
	h.version.swim = int(swim)

	var flags uint32 = 0

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
			flags |= flagHasTrace
		}

		/* preferred API to get last R/W status from J15 */
		if h.version.jtag >= 15 {
			flags |= flagHasGetLastRwStatus2
		}

		/* API to set SWD frequency from J22 */
		if h.version.jtag >= 22 {
			flags |= flagHasSwdSetFreq
		}

		/* API to set JTAG frequency from J24 */
		/* API to access DAP registers from J24 */
		if h.version.jtag >= 24 {
			flags |= flagHasJtagSetFreq
			flags |= flagHasDapReg
		}

		/* Quirk for read DP in JTAG mode (V2 only) from J24, fixed in J32 */
		if h.version.jtag >= 24 && h.version.jtag < 32 {
			flags |= flagQuirkJtagDpRead
		}

		/* API to read/write memory at 16 bit from J26 */
		if h.version.jtag >= 26 {
			flags |= flagHasMem16Bit
		}

		/* API required to init AP before any AP access from J28 */
		if h.version.jtag >= 28 {
			flags |= flagHasApInit
		}

		/* API required to return proper error code on close AP from J29 */
		if h.version.jtag >= 29 {
			flags |= flagFixCloseAp
		}

		/* Banked regs (DPv1 & DPv2) support from V2J32 */
		if h.version.jtag >= 32 {
			flags |= flagHasDpBankSel
		}
	case 3:
		/* all STLINK-V3 use api-v3 */
		h.version.jtagApi = jTagApiV3

		/* STLINK-V3 is a superset of ST-LINK/V2 */

		/* API for trace */
		/* API for target voltage */
		flags |= flagHasTrace

		/* preferred API to get last R/W status */
		flags |= flagHasGetLastRwStatus2

		/* API to access DAP registers */
		flags |= flagHasDapReg

		/* API to read/write memory at 16 bit */
		flags |= flagHasMem16Bit

		/* API required to init AP before any AP access */
		flags |= flagHasApInit

		/* API required to return proper error code on close AP */
		flags |= flagFixCloseAp

		/* Banked regs (DPv1 & DPv2) support from V3J2 */
		if h.version.jtag >= 2 {
			flags |= flagHasDpBankSel
		}

		/* 8bit read/write max packet size 512 bytes from V3J6 */
		if h.version.jtag >= 6 {
			flags |= flagHasRw8Bytes512
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

	serialNo, _ := h.usbDevice.SerialNumber()

	log.Debugf("Got ST-Link: %s [%s]", vStr, serialNo)

	return nil
}

/** Issue an STLINK command via USB transfer, with retries on any wait status responses.

  Works for commands where the STLINK_DEBUG status is returned in the first
  byte of the response packet. For SWIM a SWIM_READSTATUS is requested instead.

  Returns an openocd result code.
*/
func (h *StLinkHandle) usbCmdAllowRetry(buffer []byte, size uint32) error {
	var retries int = 0

	for true {
		if (h.stMode != StLinkModeDebugSwim) || retries > 0 {
			err := h.usbTransferNoErrCheck(buffer, size)
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

		err := h.usbErrorCheck()

		if err != nil {
			usbError := err.(*usbError)

			if usbError.UsbErrorCode == usbErrorWait && retries < maximumWaitRetries {
				var delayUs time.Duration = (1 << retries) * 1000

				retries++
				log.Debugf("cmdAllowRetry ERROR_WAIT, retry %d, delaying %d microseconds", retries, delayUs)
				time.Sleep(delayUs * 1000)

				continue
			}
		}

		return err
	}

	return errors.New("invalid cmd allow retry state")
}

func (h *StLinkHandle) usbAssertSrst(srst byte) error {

	/* TODO:
		* Implement SWIM debugger
	     *
		if h.st_mode == STLINK_MODE_DEBUG_SWIM {
			return stlink_swim_assert_reset(handle, srst);
		}
	*/

	if h.version.stlink == 1 {
		return errors.New("could not find rsrt command on target")
	}

	h.usbInitBuffer(transferRxEndpoint, 2)

	h.cmdbuf[h.cmdidx] = cmdDebug
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = debugApiV2DriveNrst
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = srst
	h.cmdidx++

	return h.usbCmdAllowRetry(h.databuf, 2)
}

func (h *StLinkHandle) maxBlockSize(tarAutoIncrBlock uint32, address uint32) uint32 {
	var maxTarBlock = tarAutoIncrBlock - ((tarAutoIncrBlock - 1) & address)

	if maxTarBlock == 0 {
		maxTarBlock = 4
	}

	return maxTarBlock
}

func (h *StLinkHandle) usbBlock() uint32 {
	if (h.version.flags & flagHasRw8Bytes512) > 0 {
		return v3MaxReadWrite8
	} else {
		return maxReadWrite8
	}
}
