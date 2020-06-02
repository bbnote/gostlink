// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.
package gostlink

const (
	RAM_START                          = 0x20000000
	SEGGER_RTT_MODE_NO_BLOCK_SKIP      = 0
	SEGGER_RTT_MODE_NO_BLOCK_TRIM      = 1
	SEGGER_RTT_MODE_BLOCK_IF_FIFO_FULL = 2
)

//
// Description for a circular buffer (also called "ring buffer")
// which is used as up-buffer (T->H)
//
type SeggerRttBuffer struct {
	name         uint32
	buffer       uint32
	sizeofbuffer uint32
	wrOff        uint32
	rdOff        uint32
	flags        uint32
}

//
// RTT control block which describes the number of buffers available
// as well as the configuration for each buffer
//
type SeggerRttControlBlock struct {
	acId              [16]uint8
	maxNumUpBuffers   uint32
	maxNumDownBuffers uint32
	bufferDescription *SeggerRttBuffer
}

type SeggerRttInfo struct {
	rttDescription *SeggerRttControlBlock
	offset         uint32
}

type StLink struct {
	// TODO: lowlevel USB library contexts and handles
	rttinfo SeggerRttInfo
}
