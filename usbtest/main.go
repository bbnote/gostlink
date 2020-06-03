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
	var v gostlink.Stlink_usb_version

	handle, err := gostlink.OpenStLink(gostlink.STLINK_MODE_DEBUG_SWD)

	if err == nil {
		gostlink.GetUsbVersion(handle, &v)
	}

}
