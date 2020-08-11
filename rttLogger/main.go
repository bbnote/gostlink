// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bbnote/gostlink"
	log "github.com/sirupsen/logrus"
)

var (
	exitProgram chan bool
	flagLogFile string
	flagChannel *int
	fileHandle *os.File
)

func rttDataHandler(channel int, data []byte) error {
	if channel != *flagChannel {
		return nil
	}

	if fileHandle != nil {
		fileHandle.Write(data)
	} else {
		fmt.Printf("%d: %s", channel, data)
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

	flagDevice := flag.String("Device", "STM32F030R8", "STM32-Device type")
	flagSpeed := flag.Int("Speed", 4000, "Interface speed to target device")
	flagInterface := flag.String("if", "SWD", "Interface connecting to target")
	flagChannel = flag.Int("RTTChannel", 0, "RTT channel to interface with")

	flag.Parse()

	fileHandle = nil

	if len(flag.Args()) == 1 {
		flagLogFile = flag.Args()[0]

		file, err := os.OpenFile(flag.Args()[0], os.O_APPEND|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			fileHandle = nil
			log.Fatal(err)
		}

		fileHandle = file

		defer fileHandle.Close()
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

	err = stLink.InitializeRtt(16, gostlink.DefaultRamStart)
	if err != nil {
		log.Error(err)
	}

	exitLoop := false

	for ; exitLoop == false;  {

		err := stLink.UpdateRttChannels(false)

		if err != nil {
			log.Error(err)

		}

		err = stLink.ReadRttChannels(rttDataHandler)

		if err != nil {
			log.Error(err)
		}

		select {
		case <- exitProgram:
			exitLoop = true
		default:

		}

		time.Sleep(50 * 1000 * 1000)
	}

	stLink.Close()
	gostlink.CloseUSB()
}
