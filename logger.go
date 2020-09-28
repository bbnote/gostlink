// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/x-cray/logrus-prefixed-formatter"
)

const MaxLogLevel = logrus.DebugLevel

func init() {
	formatter := &prefixed.TextFormatter{
		DisableColors:   false,
		TimestampFormat: "15:04:05",
		FullTimestamp:   true,
		ForceFormatting: true,
	}

	logrus.SetFormatter(formatter)

	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(MaxLogLevel)
}
