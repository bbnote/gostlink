// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

// this code is mainly inspired and based on the openocd project source code
// for detailed information see

// https://sourceforge.net/p/openocd/code

package gostlink

type StLinkMode uint8 // stlink debug modes

const (
	StLinkModeUnknown   StLinkMode = 0
	StLinkModeDfu                  = 1
	StLinkModeMass                 = 2
	StLinkModeDebugJtag            = 3
	StLinkModeDebugSwd             = 4
	StLinkModeDebugSwim            = 5
)

type MemoryBlockSize int // block size for read and write operations

const (
	Memory8BitBlock  MemoryBlockSize = 1
	Memory16BitBlock                 = 2
	Memory32BitBlock                 = 4
)

// StLink property flags
const (
	flagHasTrace            = 0x01
	flagHasTargetVolt       = flagHasTrace
	flagHasSwdSetFreq       = 0x02
	flagHasJtagSetFreq      = 0x04
	flagHasMem16Bit         = 0x08
	flagHasGetLastRwStatus2 = 0x10
	flagHasDapReg           = 0x20
	flagQuirkJtagDpRead     = 0x40
	flagHasApInit           = 0x80
	flagHasDpBankSel        = 0x100
	flagHasRw8Bytes512      = 0x200
	flagFixCloseAp          = 0x400
)

type stLinkApiVersion uint8 // api versions of stlinks

const (
	jTagApiV1 stLinkApiVersion = 1
	jTagApiV2                  = 2
	jTagApiV3                  = 3
)

// usb endpoint definitions
const (
	usbEndpointIn  = 0x80
	usbEndpointOut = 0x00

	usbWriteTimeoutMs = 1000
	usbReadTimeoutMs  = 1000

	usbRxEndpointNo    = 1 | usbEndpointIn
	usbTxEndpointNo    = 2 | usbEndpointOut
	usbTraceEndpointNo = 3 | usbEndpointIn

	usbTxEndpointApi2v1    = 1 | usbEndpointOut
	usbTraceEndpointApi2v1 = 2 | usbEndpointIn
)

// stlink internal device mode numbers
const (
	deviceModeDFU        = 0x00
	deviceModeMass       = 0x01
	deviceModeDebug      = 0x02
	deviceModeSwim       = 0x03
	deviceModeBootloader = 0x04
	deviceModeUnknown    = -1
)

type usbTransferEndpoint uint8

const (
	transferRxEndpoint    usbTransferEndpoint = 0
	transferTxEndpoint                        = 1
	transferTraceEndpoint                     = 2
)

const (
	swimErrorOk                  = 0x00
	swimErrorBusy                = 0x01
	debugErrorOk                 = 0x80
	debugErrorFault              = 0x81
	jTagGetIdCodeError           = 0x09
	jTagWriteError               = 0x0c
	jTagWriteVerifyError         = 0x0d
	swdAccessPortWait            = 0x10
	swdAccessPortFault           = 0x11
	swdAccessPortError           = 0x12
	swdAccessPortParityError     = 0x13
	swdDebugPortWait             = 0x14
	swdDebugPortFault            = 0x15
	swdDebugPortError            = 0x16
	swdDebugPortParityError      = 0x17
	swdAccessPortWDataError      = 0x18
	swdAccessPortStickyError     = 0x19
	swdAccessPortStickOrRunError = 0x1a
	badAccessPortError           = 0x1d
)

// states of cpu which is connected to stlink
const (
	debugCoreRunning       = 0x80
	debugCoreHalted        = 0x81
	debugCoreStatusUnknown = -1
)

const (
	stLinkV1Pid          = 0x3744
	stLinkV2Pid          = 0x3748
	stLinkV21Pid         = 0x374B
	stLinkV21NoMsdPid    = 0x3752
	stLinkV3UsbLoaderPid = 0x374D
	stLinkV3EPid         = 0x374E
	stLinkV3SPid         = 0x374F
	stLinkV32VcpPid      = 0x3753
)

const (
	cmdRequestSense     = 0x03
	cmdGetVersion       = 0xF1
	cmdDebug            = 0xF2
	cmdDfu              = 0xF3
	cmdSwim             = 0xF4
	cmdGetCurrentMode   = 0xF5
	cmdGetTargetVoltage = 0xF7
)

const (
	//STLINK_DEBUG_GETSTATUS           = 0x01
	//STLINK_DEBUG_FORCEDEBUG          = 0x02
	//STLINK_DEBUG_APIV1_RESETSYS      = 0x03
	//STLINK_DEBUG_APIV1_READALLREGS   = 0x04
	//STLINK_DEBUG_APIV1_READREG       = 0x05
	//STLINK_DEBUG_APIV1_WRITEREG      = 0x06
	debugReadMem32Bit  = 0x07
	debugWriteMem32Bit = 0x08
	//STLINK_DEBUG_RUNCORE             = 0x09
	//STLINK_DEBUG_STEPCORE            = 0x0a
	//STLINK_DEBUG_APIV1_SETFP         = 0x0b
	debugReadMem8Bit  = 0x0c
	debugWriteMem8Bit = 0x0d
	//STLINK_DEBUG_APIV1_CLEARFP       = 0x0e
	//STLINK_DEBUG_APIV1_WRITEDEBUGREG = 0x0f
	//STLINK_DEBUG_APIV1_SETWATCHPOINT = 0x10
	//STLINK_DEBUG_ENTER_JTAG_RESET    = 0x00
	debugEnterSwdNoReset  = 0xa3
	debugEnterJTagNoReset = 0xa4
	debugApiV1Enter       = 0x20
	debugExit             = 0x21
	debugReadCoreId       = 0x22
	debugApiV2Enter       = 0x30
	debugApiV2ReadIdCodes = 0x31
	//STLINK_DEBUG_APIV2_RESETSYS      = 0x32
	//STLINK_DEBUG_APIV2_READREG       = 0x33
	//STLINK_DEBUG_APIV2_WRITEREG      = 0x34
	//STLINK_DEBUG_APIV2_WRITEDEBUGREG = 0x35
	//STLINK_DEBUG_APIV2_READDEBUGREG  = 0x36
	//STLINK_DEBUG_APIV2_READALLREGS     = 0x3A
	debugApiV2GetLastRWStatus              = 0x3B
	debugApiV2DriveNrst                    = 0x3C
	debugApiV2GetLastRWStatus2             = 0x3E
	debugApiV2StartTraceRx                 = 0x40
	debugApiV2StopTraceRx                  = 0x41
	debugApiV2GetTraceNB                   = 0x42
	debugApiV2SwdSetFreq                   = 0x43
	debugApiV2JTagSetFreq                  = 0x44
	debugApiV2ReadDebugAccessPortRegister  = 0x45
	debugApiV2WriteDebugAccessPortRegister = 0x46
	debugApiV2ReadMem16Bit                 = 0x47
	debugApiV2WriteMem16Bit                = 0x48
	debugApiV2InitAccessPort               = 0x4B
	debugApiV2CloseAccessPortDbg           = 0x4C
	//STLINK_DEBUG_APIV2_DRIVE_NRST_LOW   = 0x00
	//STLINK_DEBUG_APIV2_DRIVE_NRST_HIGH  = 0x01
	//STLINK_DEBUG_APIV2_DRIVE_NRST_PULSE = 0x02

	debugApiV3SetComFreq   = 0x61
	debugApiV3GetComFreq   = 0x62
	debugApiV3GetVersionEx = 0xFB
)

const (
	dfuExit = 0x07
)

const (
	swimEnter = 0x00
	swimExit  = 0x01
	//STLINK_SWIM_READ_CAP       = 0x02
	//STLINK_SWIM_SPEED          = 0x03
	//STLINK_SWIM_ENTER_SEQ      = 0x04
	//STLINK_SWIM_GEN_RST        = 0x05
	//STLINK_SWIM_RESET          = 0x06
	//STLINK_SWIM_ASSERT_RESET   = 0x07
	//STLINK_SWIM_DEASSERT_RESET = 0x08
	//STLINK_SWIM_READSTATUS     = 0x09
	//STLINK_SWIM_WRITEMEM       = 0x0a
	//STLINK_SWIM_READMEM        = 0x0b
	//STLINK_SWIM_READBUF        = 0x0c
)

const (
	requestSenseLength = 18
)

const (
	maximumWaitRetries              = 8
	debugAccessPortSelectionMaximum = 255

	cpuIdBaseRegister = 0xE000ED00

	maxReadWrite8   = 64
	v3MaxReadWrite8 = 512
	v3MaxFreqNb     = 10

	cmdBufferSize  = 31
	dataBufferSize = 4096
	//cmdSizeV1        = 10
	cmdSizeV2 = 16

	traceSize  = 4096
	traceMaxHz = 2000000

	//STLINK_DEBUG_PORT_ACCESS = 0xffff
	//STLINK_SERIAL_LEN  = 24
)
