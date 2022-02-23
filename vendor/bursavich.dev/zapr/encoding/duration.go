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

// A DurationEncoder provides a named zapcore.DurationEncoder.
type DurationEncoder interface {
	DurationEncoder() zapcore.DurationEncoder
	Name() string
}

var durationEncoders = make(map[string]DurationEncoder)

// RegisterDurationEncoder registers the DurationEncoder for use as a flag argument.
func RegisterDurationEncoder(e DurationEncoder) error {
	name := e.Name()
	if _, ok := durationEncoders[name]; ok {
		return fmt.Errorf("zapr: already registered DurationEncoder: %q", name)
	}
	durationEncoders[name] = e
	return nil
}

// DurationEncoders returns the registered DurationEncoders.
func DurationEncoders() []DurationEncoder {
	s := make([]DurationEncoder, 0, len(durationEncoders))
	for _, e := range durationEncoders {
		s = append(s, e)
	}
	sort.Slice(s, func(i, k int) bool { return s[i].Name() < s[k].Name() })
	return s
}

type durationEncoder struct {
	e    zapcore.DurationEncoder
	name string
}

func (e *durationEncoder) DurationEncoder() zapcore.DurationEncoder { return e.e }
func (e *durationEncoder) Name() string                             { return e.name }

var (
	stringDurationEncoder = DurationEncoder(&durationEncoder{name: "string", e: zapcore.StringDurationEncoder})
	nanosDurationEncoder  = DurationEncoder(&durationEncoder{name: "nanos", e: zapcore.NanosDurationEncoder})
	millisDurationEncoder = DurationEncoder(&durationEncoder{name: "millis", e: zapcore.MillisDurationEncoder})
	secsDurationEncoder   = DurationEncoder(&durationEncoder{name: "secs", e: zapcore.SecondsDurationEncoder})
)

func init() {
	must(RegisterDurationEncoder(stringDurationEncoder))
	must(RegisterDurationEncoder(nanosDurationEncoder))
	must(RegisterDurationEncoder(millisDurationEncoder))
	must(RegisterDurationEncoder(secsDurationEncoder))
}

// StringDurationEncoder serializes a time.Duration using its String method.
func StringDurationEncoder() DurationEncoder { return stringDurationEncoder }

// NanosecondsDurationEncoder serializes a time.Duration to an integer number of nanoseconds.
func NanosecondsDurationEncoder() DurationEncoder { return nanosDurationEncoder }

// MillisecondsDurationEncoder serializes a time.Duration to a floating-point number of milliseconds.
func MillisecondsDurationEncoder() DurationEncoder { return millisDurationEncoder }

// SecondsDurationEncoder serializes a time.Duration to a floating-point number of seconds.
func SecondsDurationEncoder() DurationEncoder { return secsDurationEncoder }

type durationEncoderFlag struct {
	e *DurationEncoder
}

// DurationEncoderFlag returns a flag value for the encoder.
func DurationEncoderFlag(encoder *DurationEncoder) flag.Value {
	return &durationEncoderFlag{encoder}
}

func (f *durationEncoderFlag) Get() interface{} { return *f.e }
func (f *durationEncoderFlag) Set(s string) error {
	if e, ok := durationEncoders[s]; ok {
		*f.e = e
		return nil
	}
	return fmt.Errorf("zapr: unknown DurationEncoder: %q", s)
}
func (f *durationEncoderFlag) String() string {
	if f.e == nil {
		return ""
	}
	return (*f.e).Name()
}
