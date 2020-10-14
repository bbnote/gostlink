// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"bytes"

	"github.com/google/gousb"
)

func idExists(slice []gousb.ID, item gousb.ID) bool {
	for _, element := range slice {
		if element == item {
			return true
		}
	}

	return false
}

func memset(a []uint8, size int, v uint8) {
	for i := 0; i < size; i++ {
		a[i] = v
	}
}

// Adds an uint32 to current buffer
// it's also possible to determine bit position in buffer and amount of bits to be set

func addU32ToBuffer(buffer *bytes.Buffer, firstBit uint, numBits uint, value uint32) {

	if (numBits == 32) && (firstBit == 0) {
		buffer.WriteByte(uint8((value >> 0) & 0xff))
		buffer.WriteByte(uint8((value >> 8) & 0xff))
		buffer.WriteByte(uint8((value >> 16) & 0xff))
		buffer.WriteByte(uint8((value >> 24) & 0xff))

	} else {
		logger.Panic("Implement bit position setting in addU32ToBuffer")
		/*
			for i := firstBit; i < firstBit+numBits; i++ {
				if ((value >> (i - firstBit)) & 1) == 1 {
					buffer[i/8] |= 1 << (i % 8)
				} else {
					buffer[i/8] &= ^(1 << (i % 8))
				}
			}*/
	}
}

func buf_get_u32(buffer []byte, first uint, num uint) uint32 {
	if (num == 32) && (first == 0) {
		return (uint32(buffer[3]) << 24) |
			(uint32(buffer[2]) << 16) |
			(uint32(buffer[1]) << 8) |
			(uint32(buffer[0]) << 0)
	} else {
		var result uint32 = 0
		for i := first; i < first+num; i++ {
			if ((buffer[i/8] >> (i % 8)) & 1) == 1 {
				result |= uint32(1) << (i - first)
			}
		}
		return result
	}
}
