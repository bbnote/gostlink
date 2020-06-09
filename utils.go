// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import "github.com/google/gousb"

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

func buf_set_u32(buffer []uint8, first uint, num uint, value uint32) {

	if (num == 32) && (first == 0) {
		buffer[3] = uint8((value >> 24) & 0xff)
		buffer[2] = uint8((value >> 16) & 0xff)
		buffer[1] = uint8((value >> 8) & 0xff)
		buffer[0] = uint8((value >> 0) & 0xff)
	} else {
		for i := first; i < first+num; i++ {
			if ((value >> (i - first)) & 1) == 1 {
				buffer[i/8] |= 1 << (i % 8)
			} else {
				buffer[i/8] &= ^(1 << (i % 8))
			}
		}
	}
}

func buf_get_u32(buffer []byte, first uint, num uint) uint32 {
	if (num == 32) && (first == 0) {
		return ((uint32(buffer[3]) << 24) |
			(uint32(buffer[2]) << 16) |
			(uint32(buffer[1]) << 8) |
			(uint32(buffer[0]) << 0))
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

func be_to_h_u16(buffer []byte) uint16 {
	return uint16(uint16(buffer[1]) | (uint16(buffer[0]) << 8))
}

func le_to_h_u16(buffer []byte) uint16 {
	return uint16(uint16(buffer[0]) | (uint16(buffer[1]) << 8))
}

func le_to_h_u32(buffer []byte) uint32 {
	return (uint32(buffer[0]) | uint32(buffer[1])<<8 | uint32(buffer[2])<<16 | uint32(buffer[3])<<24)
}

func uint32ToLittleEndian(buffer []byte, value uint32) {
	buffer[3] = byte(value >> 24)
	buffer[2] = byte(value >> 16)
	buffer[1] = byte(value >> 8)
	buffer[0] = byte(value >> 0)
}

func uint16ToLittleEndian(buffer []byte, value uint16) {
	buffer[1] = byte(value >> 8)
	buffer[0] = byte(value >> 0)
}
