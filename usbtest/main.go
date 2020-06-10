// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/bbnote/gostlink"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.Info("Starting usb stlink test-software...")

	err := gostlink.InitializeUSB()

	if err != nil {
		log.Panic(err)
	}

	config := gostlink.NewStLinkConfig(gostlink.STLINK_ALL_VIDS, gostlink.STLINK_ALL_PIDS, gostlink.STLINK_MODE_DEBUG_SWD,
		"", 24000, false)

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

	stlink.Close()
	gostlink.CloseUSB()
}
