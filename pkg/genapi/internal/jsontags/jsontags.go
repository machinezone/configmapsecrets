// Copyright 2020 Machine Zone, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsontags

import (
	"strings"
)

// A Tag represents a struct field's json tag.
type Tag struct {
	Name    string
	Options string
}

// Parse splits a struct field's json tag into its name and
// comma-separated options.
func Parse(tag string) Tag {
	if idx := strings.Index(tag, ","); idx != -1 {
		return Tag{
			Name:    tag[:idx],
			Options: tag[idx+1:],
		}
	}
	return Tag{Name: tag}
}

// Contains reports whether the tag's comma-separated list of
// options contains a particular option.
func (t Tag) Contains(option string) bool {
	for s := string(t.Options); s != ""; {
		var next string
		if i := strings.Index(s, ","); i >= 0 {
			s, next = s[:i], s[i+1:]
		}
		if s == option {
			return true
		}
		s = next
	}
	return false
}
