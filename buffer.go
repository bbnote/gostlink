// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"bytes"
	"math"
)

type Buffer struct {
	bytes.Buffer
}

type Endian uint8

const (
	littleEndian Endian = 0
	bigEndian           = 1
)

func (e Endian) toString() string {
	if e == littleEndian {
		return "little endian"
	} else {
		return "big endian"
	}
}

func NewBuffer(initSize int) *Buffer {
	b := &Buffer{}

	b.Grow(initSize)

	return b
}

func (buf *Buffer) WriteUint32LE(value uint32) {
	buf.WriteByte(byte(value))
	buf.WriteByte(byte(value >> 8))
	buf.WriteByte(byte(value >> 16))
	buf.WriteByte(byte(value >> 24))
}

func (buf *Buffer) WriteUint16LE(value uint16) {
	buf.WriteByte(byte(value))
	buf.WriteByte(byte(value >> 8))
}

func (buf *Buffer) ReadUint16BE() uint16 {
	return convertToUint16(buf.Bytes(), bigEndian)
}

func (buf *Buffer) ReadUint16LE() uint16 {
	return convertToUint16(buf.Bytes(), littleEndian)
}

func (buf *Buffer) ReadUint32BE() uint32 {
	return convertToUint32(buf.Bytes(), bigEndian)
}

func (buf *Buffer) ReadUint32LE() uint32 {
	return convertToUint32(buf.Bytes(), littleEndian)
}

func convertToUint16(buf []byte, e Endian) uint16 {
	if len(buf) > 1 {

		if e == littleEndian {
			return uint16(buf[0]) | (uint16(buf[1]) << 8)
		} else {
			return uint16(buf[1]) | (uint16(buf[0]) << 8)
		}
	} else {
		logger.Errorf("could not read uint16 %s from given buffer", e.toString())
		return math.MaxUint16
	}
}

func convertToUint32(buf []byte, e Endian) uint32 {
	if len(buf) > 3 {

		if e == littleEndian {
			return uint32(buf[0]) | (uint32(buf[1]) << 8) | (uint32(buf[2]) << 16) | (uint32(buf[3]) << 24)
		} else {
			return uint32(buf[3]) | (uint32(buf[2]) << 8) | (uint32(buf[1]) << 16) | (uint32(buf[0]) << 24)
		}
	} else {
		logger.Errorf("could not read uint32 %s from given buffer", e.toString())
		return math.MaxUint32
	}
}
