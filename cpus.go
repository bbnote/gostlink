// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

// this code is mainly inspired and based on the openocd project source code
// for detailed information see

// https://sourceforge.net/p/openocd/code

package gostlink

type StmCpuInfo struct {
	RamStart uint64
	RamSize  uint64
}

var supportedStmCpus = map[string]StmCpuInfo{
	"STM32F030F4": {0x20000000, 0x1000},
	"STM32F030K6": {0x20000000, 0x1000},
	"STM32F030C6": {0x20000000, 0x1000},
	"STM32F030C8": {0x20000000, 0x2000},
	"STM32F030R8": {0x20000000, 0x2000},
	"STM32F030CC": {0x20000000, 0x8000},
	"STM32F030RC": {0x20000000, 0x8000},
	"STM32F070F6": {0x20000000, 0x2000},
	"STM32F070C6": {0x20000000, 0x2000},
	"STM32F070CB": {0x20000000, 0x4000},
	"STM32F070RB": {0x20000000, 0x4000},
}

func GetCpuInformation(cpuId string) *StmCpuInfo {
	if val, ok := supportedStmCpus[cpuId]; ok {
		return &val
	} else {
		return nil
	}
}
