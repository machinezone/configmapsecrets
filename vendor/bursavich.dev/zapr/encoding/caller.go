// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright 2020 Andy Bursavich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package encoding

import (
	"flag"
	"fmt"
	"sort"

	"go.uber.org/zap/zapcore"
)

// A CallerEncoder provides a named zapcore.CallerEncoder.
type CallerEncoder interface {
	CallerEncoder() zapcore.CallerEncoder
	Name() string
}

var callerEncoders = make(map[string]CallerEncoder)

// RegisterCallerEncoder registers the CallerEncoder for use as a flag argument.
func RegisterCallerEncoder(e CallerEncoder) error {
	name := e.Name()
	if _, ok := callerEncoders[name]; ok {
		return fmt.Errorf("zapr: already registered CallerEncoder: %q", name)
	}
	callerEncoders[name] = e
	return nil
}

// CallerEncoders returns the registered CallerEncoders.
func CallerEncoders() []CallerEncoder {
	s := make([]CallerEncoder, 0, len(callerEncoders))
	for _, e := range callerEncoders {
		s = append(s, e)
	}
	sort.Slice(s, func(i, k int) bool { return s[i].Name() < s[k].Name() })
	return s
}

type callerEncoder struct {
	e    zapcore.CallerEncoder
	name string
}

func (e *callerEncoder) CallerEncoder() zapcore.CallerEncoder { return e.e }
func (e *callerEncoder) Name() string                         { return e.name }

var (
	shortCallerEncoder = CallerEncoder(&callerEncoder{name: "short", e: zapcore.ShortCallerEncoder})
	fullCallerEncoder  = CallerEncoder(&callerEncoder{name: "full", e: zapcore.FullCallerEncoder})
)

func init() {
	must(RegisterCallerEncoder(shortCallerEncoder))
	must(RegisterCallerEncoder(fullCallerEncoder))
}

// ShortCallerEncoder serializes a caller in package/file:line format, trimming
// all but the final directory from the full path.
func ShortCallerEncoder() CallerEncoder { return shortCallerEncoder }

// FullCallerEncoder serializes a caller in /full/path/to/package/file:line
// format.
func FullCallerEncoder() CallerEncoder { return fullCallerEncoder }

type callerEncoderFlag struct {
	e *CallerEncoder
}

// CallerEncoderFlag returns a flag value for the encoder.
func CallerEncoderFlag(encoder *CallerEncoder) flag.Value {
	return &callerEncoderFlag{encoder}
}

func (f *callerEncoderFlag) Get() interface{} { return *f.e }
func (f *callerEncoderFlag) Set(s string) error {
	if e, ok := callerEncoders[s]; ok {
		*f.e = e
		return nil
	}
	return fmt.Errorf("zapr: unknown CallerEncoder: %q", s)
}
func (f *callerEncoderFlag) String() string {
	if f.e == nil {
		return ""
	}
	return (*f.e).Name()
}
