// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
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
		usbCtx.Debug(2)

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

	if err == nil {
		log.Infof("Found %d matching devices based on vendor and product id list", len(devices))
		return devices, nil
	} else {
		log.Error("Got error during usb device scan", err)
		return nil, err
	}
}

func usbWrite(endpoint *gousb.OutEndpoint, buffer []byte) (int, error) {
	bytesWritten, err := endpoint.Write(buffer)

	if err != nil {
		return -1, err
	} else {
		log.Tracef("Wrote %d bytes to endpoint", bytesWritten)
		return bytesWritten, nil
	}
}

func usbRead(endpoint *gousb.InEndpoint, buffer []byte) (int, error) {
	bytesRead, err := endpoint.Read(buffer)

	if err != nil {
		return -1, err
	} else {
		log.Tracef("Read %d byte from in endpoint", bytesRead)
		return bytesRead, nil
	}
}

func (h *StLinkHandle) usbGetVersion() error {
	var v, x, y, jtag, swim, msd, bridge byte = 0, 0, 0, 0, 0, 0, 0

	h.usbInitBuffer(h.rx_ep, 6)

	h.cmdbuf[h.cmdidx] = STLINK_GET_VERSION
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
	case STLINK_V2_1_PID, STLINK_V2_1_NO_MSD_PID:
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
		h.usbInitBuffer(h.rx_ep, 16)

		h.cmdbuf[h.cmdidx] = STLINK_APIV3_GET_VERSION_EX
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
			h.version.jtag_api = STLINK_JTAG_API_V2
		} else {
			h.version.jtag_api = STLINK_JTAG_API_V1
		}
	case 2:
		/* all ST-LINK/V2 and ST-Link/V2.1 use api-v2 */
		h.version.jtag_api = STLINK_JTAG_API_V2

		/* API for trace from J13 */
		/* API for target voltage from J13 */
		if h.version.jtag >= 13 {
			flags |= STLINK_F_HAS_TRACE
		}

		/* preferred API to get last R/W status from J15 */
		if h.version.jtag >= 15 {
			flags |= STLINK_F_HAS_GETLASTRWSTATUS2
		}

		/* API to set SWD frequency from J22 */
		if h.version.jtag >= 22 {
			flags |= STLINK_F_HAS_SWD_SET_FREQ
		}

		/* API to set JTAG frequency from J24 */
		/* API to access DAP registers from J24 */
		if h.version.jtag >= 24 {
			flags |= STLINK_F_HAS_JTAG_SET_FREQ
			flags |= STLINK_F_HAS_DAP_REG
		}

		/* Quirk for read DP in JTAG mode (V2 only) from J24, fixed in J32 */
		if h.version.jtag >= 24 && h.version.jtag < 32 {
			flags |= STLINK_F_QUIRK_JTAG_DP_READ
		}

		/* API to read/write memory at 16 bit from J26 */
		if h.version.jtag >= 26 {
			flags |= STLINK_F_HAS_MEM_16BIT
		}

		/* API required to init AP before any AP access from J28 */
		if h.version.jtag >= 28 {
			flags |= STLINK_F_HAS_AP_INIT
		}

		/* API required to return proper error code on close AP from J29 */
		if h.version.jtag >= 29 {
			flags |= STLINK_F_FIX_CLOSE_AP
		}

		/* Banked regs (DPv1 & DPv2) support from V2J32 */
		if h.version.jtag >= 32 {
			flags |= STLINK_F_HAS_DPBANKSEL
		}
	case 3:
		/* all STLINK-V3 use api-v3 */
		h.version.jtag_api = STLINK_JTAG_API_V3

		/* STLINK-V3 is a superset of ST-LINK/V2 */

		/* API for trace */
		/* API for target voltage */
		flags |= STLINK_F_HAS_TRACE

		/* preferred API to get last R/W status */
		flags |= STLINK_F_HAS_GETLASTRWSTATUS2

		/* API to access DAP registers */
		flags |= STLINK_F_HAS_DAP_REG

		/* API to read/write memory at 16 bit */
		flags |= STLINK_F_HAS_MEM_16BIT

		/* API required to init AP before any AP access */
		flags |= STLINK_F_HAS_AP_INIT

		/* API required to return proper error code on close AP */
		flags |= STLINK_F_FIX_CLOSE_AP

		/* Banked regs (DPv1 & DPv2) support from V3J2 */
		if h.version.jtag >= 2 {
			flags |= STLINK_F_HAS_DPBANKSEL
		}

		/* 8bit read/write max packet size 512 bytes from V3J6 */
		if h.version.jtag >= 6 {
			flags |= STLINK_F_HAS_RW8_512BYTES
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

	serialNo, _ := h.usb_device.SerialNumber()

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
		if (h.st_mode != STLINK_MODE_DEBUG_SWIM) || retries > 0 {
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
			usbError := err.(*UsbError)

			if usbError.UsbErrorCode == ErrorWait && retries < MAX_WAIT_RETRIES {
				var delayUs time.Duration = (1 << retries) * 1000

				retries++
				log.Debugf("cmdAllowRetry ERROR_WAIT, retry %d, delaying %u microseconds", retries, delayUs)
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

	h.usbInitBuffer(h.rx_ep, 2)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++
	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_DRIVE_NRST
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
	if (h.version.flags & STLINK_F_HAS_RW8_512BYTES) > 0 {
		return STLINKV3_MAX_RW8
	} else {
		return STLINK_MAX_RW8
	}
}
