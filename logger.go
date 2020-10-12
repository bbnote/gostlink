// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

import (
	"github.com/sirupsen/logrus"
)

var (
	logger *logrus.Logger = nil
)

const MaxLogLevel = logrus.DebugLevel

func init() {
	logger = logrus.New()
}

func SetLogger(loggerInstance *logrus.Logger) {

	logger = loggerInstance
}
