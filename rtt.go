// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

// based on https://github.com/phryniszak/strtt

package gostlink

import (
	"bytes"
	"errors"
	"sort"
)

type RttDataCb func(int, []byte) error

const (
	DefaultRamStart = 0x20000000
)

type seggerRttMode int

const (
	SeggerRttModeNoBlockSkip     seggerRttMode = 0
	SeggerRttModeNoBlockTrim                   = 1
	SeggerRttModeBlockIfFifoFull               = 2
)

// hold size of data structs to avoid working with sizeof (from unsafe package)
const (
	seggerRttBufferSize       = 24
	seggerRttControlBlockSize = 24
)

// all data that belongs to a Segger RTT channel (up- or down stream)
//
type seggerRttChannel struct {
	name         uint32 // pointer to name
	buffer       uint32 // pointer to start of buffer
	sizeOfBuffer uint32
	wrOff        uint32
	rdOff        uint32
	flags        uint32
}

//
// RTT control block which describes the number of buffers available
// as well as the configuration for each buffer
//
type seggerRttControlBlock struct {
	acId              [16]byte // initialized to "SEGGER RTT"
	maxNumUpBuffers   uint32
	maxNumDownBuffers uint32
	channels          []*seggerRttChannel
}

// holds information for SeggerRTT
type seggerRttInfo struct {
	offset       uint32
	ramStart     uint32
	controlBlock seggerRttControlBlock
}

func (h *StLink) InitializeRtt(rttSearchRanges [][2]uint64) error {

	for _, r := range rttSearchRanges {
		logger.Infof("searching for SeggerRTT in range  [%08x, %08x]", r[0], r[0]+r[1])

		ramStart := uint32(r[0])
		rangeSize := uint32(r[1])

		h.seggerRtt.ramStart = ramStart
		ramBuffer := bytes.NewBuffer([]byte{})

		err := h.ReadMem(ramStart, 4, rangeSize/4, ramBuffer)

		if err != nil {
			return err
		} else {
			occ := bytes.Index(ramBuffer.Bytes(), []byte("SEGGER RTT"))

			if occ != -1 {
				h.seggerRtt.offset = uint32(occ)

				logger.Infof("found RTT control block at address: 0x%08x", h.seggerRtt.ramStart+h.seggerRtt.offset)
				parseRttControlBlock(ramBuffer.Bytes()[h.seggerRtt.offset:], &h.seggerRtt.controlBlock)

				if h.seggerRtt.controlBlock.maxNumDownBuffers == 0 || h.seggerRtt.controlBlock.maxNumUpBuffers == 0 {
					return errors.New("could not find any up or downstream buffers in rtt block")
				} else {
					logger.Debugf("got AC-ID: %s, MaxNumUpBuffers: %d, MaxNumDownBuffers: %d",
						h.seggerRtt.controlBlock.acId,
						h.seggerRtt.controlBlock.maxNumUpBuffers,
						h.seggerRtt.controlBlock.maxNumDownBuffers)

					h.seggerRtt.controlBlock.channels = make([]*seggerRttChannel, h.seggerRtt.controlBlock.maxNumUpBuffers+
						h.seggerRtt.controlBlock.maxNumDownBuffers)

					return nil
				}
			} else {
				logger.Warn("could not find Segger RTT control block id in this range")
			}
		}
	}

	return errors.New("could not find any rtt control block in given ranges")

}

func (h *StLink) UpdateRttChannels(readChannelNames bool) error {
	bufferAmount := h.seggerRtt.controlBlock.maxNumUpBuffers + h.seggerRtt.controlBlock.maxNumDownBuffers
	ramBuffer := bytes.NewBuffer([]byte{})
	size := bufferAmount * seggerRttBufferSize

	err := h.ReadMem(h.seggerRtt.ramStart+h.seggerRtt.offset+seggerRttControlBlockSize, 1, size, ramBuffer)

	if err == nil {
		controlBlockOffset := uint32(0)

		ramBytes := ramBuffer.Bytes()

		for i := uint32(0); i < bufferAmount; i++ {
			rttBuffer := &seggerRttChannel{}

			rttBuffer.name = convertToUint32(ramBytes[controlBlockOffset:], littleEndian)
			controlBlockOffset += 4

			rttBuffer.buffer = convertToUint32(ramBytes[controlBlockOffset:], littleEndian)
			controlBlockOffset += 4

			rttBuffer.sizeOfBuffer = convertToUint32(ramBytes[controlBlockOffset:], littleEndian)
			controlBlockOffset += 4

			rttBuffer.wrOff = convertToUint32(ramBytes[controlBlockOffset:], littleEndian)
			controlBlockOffset += 4

			rttBuffer.rdOff = convertToUint32(ramBytes[controlBlockOffset:], littleEndian)
			controlBlockOffset += 4

			rttBuffer.flags = convertToUint32(ramBytes[controlBlockOffset:], littleEndian)
			controlBlockOffset += 4

			if rttBuffer.name != 0 && readChannelNames == true {
				channelNameBuf := bytes.NewBuffer([]byte{})

				h.ReadMem(rttBuffer.name, 1, 64, channelNameBuf)
				channelName, _ := channelNameBuf.ReadString(byte(0))

				logger.Debugf("%d. Channel Name: %s, \tsize: %d, flags: %d, pBuffer 0x%08x, rdOff: %d, wrOff: %d", i,
					channelName, rttBuffer.sizeOfBuffer, rttBuffer.flags, rttBuffer.buffer, rttBuffer.rdOff, rttBuffer.wrOff)

			} else {
				//log.Debugf("%d. -------------, \tsize: %d, flags: %d, pBuffer 0x%08x,  rdOff: %d, wrOff: %d", i,
				//	rttBuffer.sizeOfBuffer, rttBuffer.flags, rttBuffer.buffer, rttBuffer.rdOff, rttBuffer.wrOff)
			}

			h.seggerRtt.controlBlock.channels[i] = rttBuffer
		}
	} else {
		return err
	}

	return nil
}

func (h *StLink) ReadRttChannels(callback RttDataCb) error {
	if h.seggerRtt.controlBlock.maxNumUpBuffers == 0 {
		return errors.New("no channels for reading configured on target")
	}

	start := h.seggerRtt.offset
	buffersCnt := h.seggerRtt.controlBlock.maxNumDownBuffers + h.seggerRtt.controlBlock.maxNumUpBuffers
	size := seggerRttControlBlockSize + seggerRttBufferSize*buffersCnt

	var blocks [][2]uint32

	for _, channel := range h.seggerRtt.controlBlock.channels {

		if channel.sizeOfBuffer > 0 && channel.rdOff != channel.wrOff {
			start = channel.buffer - h.seggerRtt.ramStart
			size = channel.sizeOfBuffer

			blocks = append(blocks, [...]uint32{start, size})
		}
	}

	// now determine channel buffer ram read boundaries
	if len(blocks) == 0 {
		//log.Debug("No data to read from channel")
		return nil
	}

	sort.Slice(blocks, func(i, j int) bool {
		if blocks[i][0] < blocks[j][0] {
			return true
		} else if blocks[i][0] == blocks[j][0] && blocks[i][1] < blocks[j][1] {
			return true
		} else {
			return false
		}
	})

	start = blocks[0][0]
	size = blocks[len(blocks)-1][0] + blocks[len(blocks)-1][1] - start

	ramBuffer := bytes.NewBuffer([]byte{})
	err := h.ReadMem(h.seggerRtt.ramStart+start, Memory8BitBlock, size, ramBuffer)

	if err != nil {
		return err
	}

	for i, channel := range h.seggerRtt.controlBlock.channels {
		if uint32(i) >= h.seggerRtt.controlBlock.maxNumUpBuffers {
			break
		}

		if (channel.sizeOfBuffer > 0) && channel.rdOff != channel.wrOff {
			channelData := bytes.NewBuffer([]byte{})
			h.readDataFromRttChannelBuffer(uint32(i), ramBuffer.Bytes(), channelData)

			callback(i, channelData.Bytes())
		}
	}

	return nil
}

func (h *StLink) readDataFromRttChannelBuffer(channelIdx uint32, ramBuffer []byte, data *bytes.Buffer) (int, error) {
	rttBuffer := h.seggerRtt.controlBlock.channels[channelIdx]
	wrOff := rttBuffer.wrOff
	RdOff := rttBuffer.rdOff

	// determine buffer index
	bufferOffset := uint32(0)
	for i, channel := range h.seggerRtt.controlBlock.channels {
		if uint32(i) >= channelIdx {
			break
		}
		bufferOffset += channel.sizeOfBuffer
	}

	for RdOff != wrOff {
		data.WriteByte(ramBuffer[bufferOffset+RdOff])
		RdOff++

		if RdOff > rttBuffer.sizeOfBuffer-1 {
			RdOff = 0

		}
	}

	if data.Len() > 0 {
		addressRdOff := h.seggerRtt.ramStart + h.seggerRtt.offset + seggerRttControlBlockSize + channelIdx*seggerRttBufferSize + 16 // 20 bytes rdOff pos

		wrBuffer := Buffer{}
		wrBuffer.WriteUint32LE(RdOff)

		err := h.WriteMem(addressRdOff, Memory32BitBlock, 1, wrBuffer.Bytes())

		if err != nil {
			return -1, err
		}
	}

	return data.Len(), nil
}

func parseRttControlBlock(ramBuffer []byte, controlBlock *seggerRttControlBlock) {
	copy(controlBlock.acId[:], ramBuffer) // is 16 bytes long
	controlBlock.maxNumUpBuffers = convertToUint32(ramBuffer[len(controlBlock.acId):], littleEndian)
	controlBlock.maxNumDownBuffers = convertToUint32(ramBuffer[len(controlBlock.acId)+4:], littleEndian)
}
