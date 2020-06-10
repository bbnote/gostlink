// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

// this code is mainly inspired and based on the openocd project source code
// for detailed information see

// https://sourceforge.net/p/openocd/code

package gostlink

import (
	"errors"
	"github.com/google/gousb"
	log "github.com/sirupsen/logrus"
	"time"
)

const STLINK_ALL_VIDS = 0xFFFF
const STLINK_ALL_PIDS = 0xFFFF

var stlink_supported_vids = []gousb.ID{0x0483} // STLINK Vendor ID
var stlink_supported_pids = []gousb.ID{0x3744, 0x3748, 0x374b, 0x374d, 0x374e, 0x374f, 0x3752, 0x3753}

type StLinkVersion struct {
	/** */
	stlink int
	/** */
	jtag int
	/** */
	swim int
	/** jtag api version supported */
	jtag_api StLinkApiVersion
	/** one bit for each feature supported. See macros STLINK_F_* */
	flags uint32
}

type StLinkTrace struct {
	enabled  bool
	sourceHz uint32
}

/** */
type StLinkHandle struct {
	/** */
	usb_device *gousb.Device
	/** */
	usb_config *gousb.Config
	/** */
	usb_interface *gousb.Interface
	/** */
	rx_ep uint8
	/** */
	tx_ep uint8
	/** */
	trace_ep uint8
	/** */
	cmdbuf []byte
	/** */
	cmdidx uint8
	/** */
	direction uint8
	/** */
	databuf []byte
	/** */
	max_mem_packet uint32
	/** */
	st_mode StLinkMode
	/** */
	version StLinkVersion

	trace StLinkTrace

	/** */
	vid gousb.ID
	/** */
	pid gousb.ID
	/** reconnect is needed next time we try to query the
	 * status */
	reconnectPending bool
}

type StLinkInterfaceConfig struct {
	vid               gousb.ID
	pid               gousb.ID
	mode              StLinkMode
	serial            string
	initialSpeed      uint32
	connectUnderReset bool
}

func NewStLinkConfig(vid gousb.ID, pid gousb.ID, mode StLinkMode,
	serial string, initialSpeed uint32, connectUnderReset bool) *StLinkInterfaceConfig {

	config := &StLinkInterfaceConfig{
		vid:               vid,
		pid:               pid,
		mode:              mode,
		serial:            serial,
		initialSpeed:      initialSpeed,
		connectUnderReset: connectUnderReset,
	}

	return config
}

func NewStLink(config *StLinkInterfaceConfig) (*StLinkHandle, error) {
	var err error
	var devices []*gousb.Device

	handle := &StLinkHandle{}
	handle.st_mode = config.mode

	// initialize data buffers for tx and rx
	handle.cmdbuf = make([]byte, STLINK_SG_SIZE)
	handle.databuf = make([]byte, STLINK_DATA_SIZE)

	if config.vid == STLINK_ALL_VIDS && config.pid == STLINK_ALL_PIDS {
		devices, err = usbFindDevices(stlink_supported_vids, stlink_supported_pids)

	} else if config.vid == STLINK_ALL_VIDS && config.pid != STLINK_ALL_PIDS {
		devices, err = usbFindDevices(stlink_supported_vids, []gousb.ID{config.pid})

	} else if config.vid != STLINK_ALL_VIDS && config.pid == STLINK_ALL_PIDS {
		devices, err = usbFindDevices([]gousb.ID{config.vid}, stlink_supported_pids)

	} else {
		devices, err = usbFindDevices([]gousb.ID{config.vid}, []gousb.ID{config.pid})
	}

	if err != nil {
		return nil, err
	}

	if len(devices) > 0 {

		if config.serial == "" && len(devices) > 1 {
			return nil, errors.New("could not identity exact stlink by given parameters. (Perhaps a serial no is missing?)")
		} else if len(devices) == 1 {
			handle.usb_device = devices[0]
		} else {
			for _, dev := range devices {
				devSerialNo, _ := dev.SerialNumber()

				log.Debugf("Compare serial no %s with number %s", devSerialNo, config.serial)

				if devSerialNo == config.serial {
					handle.usb_device = dev

					log.Infof("Found st link with serial number %s", devSerialNo)
				}
			}
		}
	} else {
		return nil, errors.New("could not find any ST-Link connected to computer")
	}

	if handle.usb_device == nil {
		return nil, errors.New("could not find ST-Link by given parameters")
	}

	// no request required configuration an matching usb interface :D
	handle.usb_config, err = handle.usb_device.Config(1)
	if err != nil {
		log.Debug(err)
		return nil, errors.New("could not request configuration #1 for st-link debugger")
	}

	handle.usb_interface, err = handle.usb_config.Interface(0, 0)
	if err != nil {
		log.Debug(err)
		return nil, errors.New("could not claim interface 0,0 for st-link debugger")
	}

	handle.rx_ep = STLINK_RX_EP // Endpoint for rx is on all st links the same

	switch handle.usb_device.Desc.Product {
	case STLINK_V1_PID:
		handle.version.stlink = 1
		handle.tx_ep = STLINK_TX_EP

	case STLINK_V3_USBLOADER_PID, STLINK_V3E_PID, STLINK_V3S_PID, STLINK_V3_2VCP_PID:
		handle.version.stlink = 3
		handle.tx_ep = STLINK_V2_1_TX_EP
		handle.trace_ep = STLINK_V2_1_TRACE_EP

	case STLINK_V2_1_PID, STLINK_V2_1_NO_MSD_PID:
		handle.version.stlink = 2
		handle.tx_ep = STLINK_V2_1_TX_EP
		handle.trace_ep = STLINK_V2_1_TRACE_EP

	default:
		log.Infof("Could not determine pid of debugger %04x. Assuming Link V2", handle.usb_device.Desc.Product)
		handle.version.stlink = 2
		handle.tx_ep = STLINK_TX_EP
		handle.trace_ep = STLINK_TRACE_EP
	}

	err = handle.usbGetVersion()

	if err != nil {
		return nil, err
	}

	switch handle.st_mode {
	case STLINK_MODE_DEBUG_SWD:
		if handle.version.jtag_api == STLINK_JTAG_API_V1 {
			return nil, errors.New("SWD not supported by jtag api v1")
		}
	case STLINK_MODE_DEBUG_JTAG:
		if handle.version.jtag == 0 {
			return nil, errors.New("JTAG transport not supported by stlink")
		}
	case STLINK_MODE_DEBUG_SWIM:
		if handle.version.swim == 0 {
			return nil, errors.New("Swim transport not supported by device")
		}

	default:
		return nil, errors.New("unknown ST-Link mode")
	}

	err = handle.usbInitMode(config.connectUnderReset, config.initialSpeed)

	if err != nil {
		return nil, err
	}

	/**
		TODO: Implement SWIM mode configuration
	if (h->st_mode == STLINK_MODE_DEBUG_SWIM) {
		err = stlink_swim_enter(h);
		if (err != ERROR_OK) {
			LOG_ERROR("stlink_swim_enter_failed (unable to connect to the target)");
			goto error_open;
		}
		*fd = h;
		h->max_mem_packet = STLINK_DATA_SIZE;
		return ERROR_OK;
	}
	*/

	handle.max_mem_packet = 1 << 10

	err = handle.usbInitAccessPort(0)

	if err != nil {
		return nil, err
	}

	buffer := make([]byte, 4)
	errCode := handle.usbReadMem32(CPUID_BASE_REGISTER, 4, buffer)

	if errCode == nil {
		var cpuid uint32 = le_to_h_u32(buffer)
		var i uint32 = (cpuid >> 4) & 0xf

		if i == 4 || i == 3 {
			/* Cortex-M3/M4 has 4096 bytes autoincrement range */
			log.Debug("Set mem packet layout according to Cortex M3/M4")
			handle.max_mem_packet = 1 << 12
		}
	}

	log.Debugf("Using TAR autoincrement: %d", handle.max_mem_packet)
	return handle, nil
}

func (h *StLinkHandle) Close() {
	if h.usb_device != nil {
		log.Debugf("Close ST-Link device [%04x:%04x]", uint16(h.vid), uint16(h.pid))

		h.usb_interface.Close()
		h.usb_config.Close()
		h.usb_device.Close()
	}
}

func (h *StLinkHandle) GetTargetVoltage() (float32, error) {
	var adcResults [2]uint32

	/* no error message, simply quit with error */
	if (h.version.flags & STLINK_F_HAS_TARGET_VOLT) == 0 {
		return -1.0, errors.New("device does not support voltage measurement")
	}

	h.usbInitBuffer(h.rx_ep, 8)

	h.cmdbuf[h.cmdidx] = STLINK_GET_TARGET_VOLTAGE
	h.cmdidx++

	err := h.usbTransferNoErrCheck(h.databuf, 8)

	if err != nil {
		return -1.0, err
	}

	/* convert result */
	adcResults[0] = le_to_h_u32(h.databuf)
	adcResults[1] = le_to_h_u32(h.databuf[4:])

	var targetVoltage float32 = 0.0

	if adcResults[0] > 0 {
		targetVoltage = 2 * (float32(adcResults[1]) * (1.2 / float32(adcResults[0])))
	}

	log.Infof("Target voltage: %f", targetVoltage)

	return targetVoltage, nil
}

func (h *StLinkHandle) GetIdCode() (uint32, error) {
	var offset int
	var retVal error

	if h.st_mode == STLINK_MODE_DEBUG_SWIM {
		return 0, nil
	}

	h.usbInitBuffer(h.rx_ep, 12)

	h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
	h.cmdidx++

	if h.version.jtag_api == STLINK_JTAG_API_V1 {
		h.cmdbuf[h.cmdidx] = STLINK_DEBUG_READCOREID
		h.cmdidx++

		retVal = h.usbTransferNoErrCheck(h.databuf, 4)
		offset = 0
	} else {
		h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_READ_IDCODES
		h.cmdidx++

		retVal = h.usbTransferErrCheck(h.databuf, 12)
		offset = 4
	}

	if retVal != nil {
		return 0, retVal

	} else {
		idCode := le_to_h_u32(h.databuf[offset:])

		return idCode, nil
	}
}
func (h *StLinkHandle) SetSpeed(khz uint32, query bool) (uint32, error) {

	switch h.st_mode {
	/*case STLINK_MODE_DEBUG_SWIM:
	return stlink_speed_swim(khz, query)
	*/

	case STLINK_MODE_DEBUG_SWD:
		if h.version.jtag_api == STLINK_JTAG_API_V3 {
			return h.setSpeedV3(false, khz, query)
		} else {
			return h.setSpeedSwd(khz, query)
		}

	/*case STLINK_MODE_DEBUG_JTAG:
	if h.version.jtag_api == STLINK_JTAG_API_V3 {
		return stlink_speed_v3(true, khz, query)
	} else {
		return stlink_speed_jtag(khz, query)
	}
	*/
	default:
		return khz, errors.New("requested ST-Link mode not supported yet")
	}
}

func (h *StLinkHandle) ConfigTrace(enabled bool, tpiuProtocol TpuiPinProtocolType, portSize uint32,
	traceFreq *uint32, traceClkInFreq uint32, preScaler *uint16) error {

	if enabled == true && ((h.version.flags&STLINK_F_HAS_TRACE == 0) || tpiuProtocol != TpuiPinProtocolAsyncUart) {
		return errors.New("the attached ST-Link version does not support this trace mode")
	}

	if !enabled {
		h.usbTraceDisable()
		return nil
	}

	if *traceFreq > STLINK_TRACE_MAX_HZ {
		return errors.New("this ST-Link version does not support frequency")
	}

	h.usbTraceDisable()

	if *traceFreq == 0 {
		*traceFreq = STLINK_TRACE_MAX_HZ
	}

	presc := uint16(traceClkInFreq / *traceFreq)

	if (traceClkInFreq % *traceFreq) > 0 {
		presc++
	}

	if presc > TpuiAcprMaxSwoScaler {
		return errors.New("SWO frequency is not suitable. Please choose a different")
	}

	*preScaler = presc
	h.trace.sourceHz = *traceFreq

	return h.usbTraceEnable()
}

func (h *StLinkHandle) ReadMem(addr uint32, size uint32, count uint32, buffer []byte) error {
	var retErr error
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
			bytesRemaining = h.maxBlockSize(h.max_mem_packet, addr)
		} else {
			bytesRemaining = h.usbBlock()
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

				err := h.usbReadMem8(addr, uint16(headBytes), buffer)

				if err != nil {
					usbError := err.(*UsbError)

					if usbError.UsbErrorCode == ErrorWait && retries < MAX_WAIT_RETRIES {
						var sleepDur time.Duration = 1 << retries
						retries++

						time.Sleep(sleepDur * 1000000)
						continue
					}

					return err
				}

				bufferPos += headBytes
				addr += headBytes
				count -= headBytes
				bytesRemaining -= headBytes
			}

			if (bytesRemaining & (size - 1)) > 0 {
				retErr = h.ReadMem(addr, 1, bytesRemaining, buffer[:bufferPos])
			} else if size == 2 {
				retErr = h.usbReadMem16(addr, uint16(bytesRemaining), buffer[:bufferPos])
			} else {
				retErr = h.usbReadMem32(addr, uint16(bytesRemaining), buffer[:bufferPos])
			}
		} else {
			retErr = h.usbReadMem8(addr, uint16(bytesRemaining), buffer[:bufferPos])
		}

		if retErr != nil {
			usbError := retErr.(*UsbError)

			if usbError.UsbErrorCode == ErrorWait && retries < MAX_WAIT_RETRIES {
				var sleepDur time.Duration = 1 << retries
				retries++

				time.Sleep(sleepDur * 1000000)
				continue
			}

			return retErr
		}

		bufferPos += bytesRemaining
		addr += bytesRemaining
		count -= bytesRemaining
	}

	return retErr
}

func (h *StLinkHandle) PollTrace(buffer []byte, size *uint32) error {

	if h.trace.enabled == true && (h.version.flags&STLINK_F_HAS_TRACE) != 0 {
		h.usbInitBuffer(h.rx_ep, 10)

		h.cmdbuf[h.cmdidx] = STLINK_DEBUG_COMMAND
		h.cmdidx++

		h.cmdbuf[h.cmdidx] = STLINK_DEBUG_APIV2_GET_TRACE_NB
		h.cmdidx++

		err := h.usbTransferNoErrCheck(h.databuf, 2)

		if err != nil {
			return err
		}

		bytesAvailable := uint32(le_to_h_u16(h.databuf))

		if bytesAvailable < *size {
			*size = bytesAvailable
		} else {
			*size = *size - 1
		}

		if *size > 0 {
			return h.usbReadTrace(buffer, *size)
		}
	}

	*size = 0
	return nil
}
