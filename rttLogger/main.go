// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bbnote/gostlink"
	log "github.com/sirupsen/logrus"
)

var (
	exitProgram chan bool
	flagLogFile string
	flagChannel *int
	fileHandle  *os.File
)

func rttDataHandler(channel int, data []byte) error {
	if channel != *flagChannel {
		return nil
	}

	if fileHandle != nil {
		fileHandle.Write(data)
	} else {
		fmt.Print(data)
	}

	return nil
}

func setUpSignalHandler() {
	signals := make(chan os.Signal, 1)
	exitProgram = make(chan bool, 1)

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signals
		exitProgram <- true
	}()

}

func main() {
	log.Info("Welcome to goST-Link library rtt logger...")

	flagDevice := flag.String("Device", "", "STM32-Device type")
	flagSpeed := flag.Int("Speed", 4000, "Interface speed to target device")
	flagInterface := flag.String("if", "SWD", "Interface connecting to target")
	flagChannel := flag.Int("RTTChannel", 0, "RTT channel to interface with")
	flagRTTAddress := flag.Uint64("RTTAddress", 0, "Sets RTT address to RTTAddress")
	flagRTTSearchRanges := flag.String("RTTSearchRanges", "", "RTTSearchRanges <RangeAddr> <RangeSize> [, <RangeAddr1> <RangeSize1>, ..]")

	flag.Parse()

	var rttSearchRanges [][2]uint64
	fileHandle = nil

	if len(flag.Args()) == 1 {
		flagLogFile = flag.Args()[0]

		file, err := os.OpenFile(flag.Args()[0], os.O_APPEND|os.O_RDWR, 0600)

		if err != nil {
			fileHandle = nil
			log.Fatal(err)
		}

		file.Truncate(0)
		file.Seek(0, 0)

		fileHandle = file

		defer fileHandle.Close()
	}

	if *flagDevice != "" {
		cpuInfo := gostlink.GetCpuInformation(*flagDevice)

		if cpuInfo != nil {
			log.Infof("Found device information for %s [0x%x, 0x%x]", *flagDevice, cpuInfo.RamStart, cpuInfo.RamSize)
			rttSearchRanges = append(rttSearchRanges, [...]uint64{cpuInfo.RamStart, cpuInfo.RamSize})

		} else {
			log.Errorf("Could not find device information for %s. Looking for RTT command line parameters...", *flagDevice)
			os.Exit(-1)
		}
	} else if *flagRTTAddress != 0 {
		rttSearchRanges = append(rttSearchRanges, [...]uint64{*flagRTTAddress, 24})

	} else if *flagRTTSearchRanges != "" {

		ranges := strings.Split(*flagRTTSearchRanges, ",")
		for _, r := range ranges {
			var rttStart uint64 = math.MaxUint64
			var rttRange uint64 = math.MaxUint64

			fmt.Sscanf(r, "%v %v", &rttStart, &rttRange)

			if rttStart != math.MaxUint64 && rttRange != math.MaxUint64 {
				log.Debugf("Adding search range [0x%x, 0x%x]", rttStart, rttRange)
				rttSearchRanges = append(rttSearchRanges, [...]uint64{rttStart, rttRange})
			} else {
				log.Warnf("Discarding invalid search range '%s'...", r)
			}
		}
	} else {
		log.Error("Could not find valid device description")
		os.Exit(-1)
	}

	err := gostlink.InitializeUSB()
	if err != nil {
		log.Panic(err)
	}

	log.Debugf("Opening target %s (%s, %d kHz) on channel %d...", *flagDevice, *flagInterface,
		*flagSpeed, *flagChannel)

	setUpSignalHandler()

	config := gostlink.NewStLinkConfig(gostlink.AllSupportedVIds, gostlink.AllSupportedPIds,
		gostlink.StLinkModeDebugSwd, "", uint32(*flagSpeed), false)

	stLink, err := gostlink.NewStLink(config)

	if stLink == nil {
		log.Fatal("Could not find any st-link on your computer")
	}

	code, err := stLink.GetIdCode()

	if err == nil {
		log.Infof("Got id code: %08x", code)
	}

	err = stLink.InitializeRtt(rttSearchRanges)
	if err != nil {
		log.Error(err)
	}

	exitLoop := false

	for exitLoop == false {

		err := stLink.UpdateRttChannels(false)

		if err != nil {
			log.Error(err)

		}

		err = stLink.ReadRttChannels(rttDataHandler)

		if err != nil {
			log.Error(err)
		}

		select {
		case <-exitProgram:
			exitLoop = true
		default:

		}

		time.Sleep(50 * 1000 * 1000)
	}

	stLink.Close()
	gostlink.CloseUSB()
}
