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
	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
)

var (
	exitProgram chan bool
	flagLogFile string
	flagChannel *int
	fileHandle  *os.File

	logger *logrus.Logger
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

func initLogger() {
	formatter := &prefixed.TextFormatter{
		DisableColors:   false,
		TimestampFormat: "15:04:05",
		FullTimestamp:   true,
		ForceFormatting: true,
	}

	logger = logrus.New()

	logger.SetFormatter(formatter)
	logger.SetOutput(os.Stdout)
}

func main() {
	initLogger()
	gostlink.SetLogger(logger)

	logger.Info("Welcome to goST-Link library rtt logger...")

	flagLogLevel := flag.Int("LogLevel", int(logrus.DebugLevel), "Logging verbosity [0 - 7]")
	flagDevice := flag.String("Device", "", "STM32-Device type")
	flagSpeed := flag.Int("Speed", 4000, "Interface speed to target device")
	flagInterface := flag.String("if", "SWD", "Interface connecting to target")
	flagChannel = flag.Int("RTTChannel", 0, "RTT channel to interface with")
	flagRTTAddress := flag.Uint64("RTTAddress", 0, "Sets RTT address to RTTAddress")
	flagRTTSearchRanges := flag.String("RTTSearchRanges", "", "RTTSearchRanges <RangeAddr> <RangeSize> [, <RangeAddr1> <RangeSize1>, ..]")

	flag.Parse()

	logger.SetLevel(logrus.Level(*flagLogLevel))

	var rttSearchRanges [][2]uint64
	fileHandle = nil

	if len(flag.Args()) == 1 {
		flagLogFile = flag.Args()[0]

		file, err := os.OpenFile(flag.Args()[0], os.O_APPEND|os.O_RDWR, 0600)

		if err != nil {
			fileHandle = nil
			logger.Fatal(err)
		}

		file.Truncate(0)
		file.Seek(0, 0)

		fileHandle = file

		defer fileHandle.Close()
	}

	if *flagDevice != "" {
		cpuInfo := gostlink.GetCpuInformation(*flagDevice)

		if cpuInfo != nil {
			logger.Infof("found device information for %s [0x%x, 0x%x]", *flagDevice, cpuInfo.RamStart, cpuInfo.RamSize)
			rttSearchRanges = append(rttSearchRanges, [...]uint64{cpuInfo.RamStart, cpuInfo.RamSize})

		} else {
			logger.Errorf("could not find device information for %s. Looking for RTT command line parameters...", *flagDevice)
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
				logger.Debugf("adding search range [0x%x, 0x%x]", rttStart, rttRange)
				rttSearchRanges = append(rttSearchRanges, [...]uint64{rttStart, rttRange})
			} else {
				logger.Warnf("discarding invalid search range '%s'...", r)
			}
		}
	} else {
		logger.Error("could not find valid device description")
		os.Exit(-1)
	}

	err := gostlink.InitUsb()
	if err != nil {
		logger.Panic(err)
	}

	logger.Debugf("searching for target %s (%s, %d kHz) with RTT on channel %d...", *flagDevice, *flagInterface,
		*flagSpeed, *flagChannel)

	setUpSignalHandler()

	config := gostlink.NewStLinkConfig(gostlink.AllSupportedVIds, gostlink.AllSupportedPIds,
		gostlink.StLinkModeDebugSwd, "", uint32(*flagSpeed), false)

	stLink, err := gostlink.NewStLink(config)

	if err != nil {
		logger.Fatal("error while scanning for st-links on your computer: ", err)
	}

	code, err := stLink.GetIdCode()

	if err == nil {
		logger.Infof("got id code: %08x", code)
	}

	err = stLink.InitializeRtt(rttSearchRanges)

	if err != nil {
		logger.Error("error during initialization of RTT: ", err)

		stLink.Close()
		gostlink.CloseUSB()

		os.Exit(-1)
	} else {
		exitLoop := false

		for exitLoop == false {

			err := stLink.UpdateRttChannels(false)

			if err != nil {
				logger.Error(err)

			}

			err = stLink.ReadRttChannels(rttDataHandler)

			if err != nil {
				logger.Error(err)
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

		os.Exit(0)
	}
}
