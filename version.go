// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"fmt"

	"github.com/boljen/go-bitmap"
	"github.com/google/gousb"
)

func (h *StLink) useParseVersion() error {
	var v, x, y, jtag, swim, msd, bridge byte = 0, 0, 0, 0, 0, 0, 0

	ctx := h.initTransfer(transferIncoming)

	ctx.cmdBuf.WriteByte(cmdGetVersion)

	err := h.usbTransferNoErrCheck(ctx, 6)

	if err != nil {
		return err
	}

	version := ctx.dataBuf.ReadUint16BE()

	v = byte((version >> 12) & 0x0f)
	x = byte((version >> 6) & 0x3f)
	y = byte(version & 0x3f)

	h.vid = gousb.ID(convertToUint16(ctx.DataBytes()[2:], littleEndian))
	h.pid = gousb.ID(convertToUint16(ctx.DataBytes()[4:], littleEndian))

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
		ctxV3 := h.initTransfer(transferIncoming)

		ctxV3.cmdBuf.WriteByte(debugApiV3GetVersionEx)

		err := h.usbTransferNoErrCheck(ctxV3, 12)

		if err != nil {
			return err
		}

		v = ctxV3.DataBytes()[0]
		swim = ctxV3.DataBytes()[1]
		jtag = ctxV3.DataBytes()[2]
		msd = ctxV3.DataBytes()[3]
		bridge = ctxV3.DataBytes()[4]
		h.vid = gousb.ID(convertToUint16(ctxV3.DataBytes()[8:], littleEndian))
		h.pid = gousb.ID(convertToUint16(ctxV3.DataBytes()[10:], littleEndian))
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
