// Copyright 2020 Sebastian Lehmann. All rights reserved.
// Use of this source code is governed by a GNU-style
// license that can be found in the LICENSE file.

package gostlink

func itemExists(slice []uint16, item uint16) bool {
	for _, element := range slice {
		if element == item {
			return true
		}
	}

	return false
}
