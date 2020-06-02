// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"os"

	"github.com/sirupsen/logrus"
)

const MaxLogLevel = logrus.TraceLevel

func init() {
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(MaxLogLevel)
}
