// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/bbnote/gostlink"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.Info("Starting usb stlink test-software...")

	err := gostlink.InitializeUSB()

	if err != nil {
		log.Panic(err)
	}

	config := gostlink.NewStLinkConfig(gostlink.AllSupportedVIds, gostlink.AllSupportedPIds,
		gostlink.StLinkModeDebugSwd, "", 10000, false)

	stlink, err := gostlink.NewStLink(config)

	if stlink != nil {
		log.Info("Found ST-Link on your computer! :)")
	} else {
		log.Fatal("Could not find any st-link on your computer")
	}

	code, err := stlink.GetIdCode()

	if err == nil {
		log.Infof("Got id code: %08x", code)
	}

	err = stlink.InitializeRtt(16, gostlink.DefaultRamStart)

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		done <- true
	}()

	exiting := false

	if err == nil {

		for i := 0; i < 30000 && exiting == false; i++ {

			err := stlink.UpdateRttChannels(false)

			if err != nil {
				log.Error(err)

			}

			err = stlink.ReadRttChannels()

			if err != nil {
				log.Error(err)
			}

			select {
			case <-done:
				exiting = true
			default:

			}

			time.Sleep(50 * 1000 * 1000)
		}

	} else {
		log.Error(err)
	}


	stlink.Close()
	gostlink.CloseUSB()
}
