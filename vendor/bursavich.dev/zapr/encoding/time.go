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
	"time"

	"go.uber.org/zap/zapcore"
)

// A TimeEncoder provides a named zapcore.TimeEncoder.
type TimeEncoder interface {
	TimeEncoder() zapcore.TimeEncoder
	Name() string
}

var timeEncoders = make(map[string]TimeEncoder)

// RegisterTimeEncoder registers the TimeEncoder for use as a flag argument.
func RegisterTimeEncoder(e TimeEncoder) error {
	name := e.Name()
	if _, ok := timeEncoders[name]; ok {
		return fmt.Errorf("zapr: already registered TimeEncoder: %q", name)
	}
	timeEncoders[name] = e
	return nil
}

// TimeEncoders returns the registered TimeEncoders.
func TimeEncoders() []TimeEncoder {
	s := make([]TimeEncoder, 0, len(timeEncoders))
	for _, e := range timeEncoders {
		s = append(s, e)
	}
	sort.Slice(s, func(i, k int) bool { return s[i].Name() < s[k].Name() })
	return s
}

type timeEncoder struct {
	e    func(time.Time, zapcore.PrimitiveArrayEncoder)
	name string
}

func (e *timeEncoder) TimeEncoder() zapcore.TimeEncoder { return e.e }
func (e *timeEncoder) Name() string                     { return e.name }

var (
	iso8601TimeEncoder = TimeEncoder(&timeEncoder{name: "iso8601", e: zapcore.ISO8601TimeEncoder})
	millisTimeEncoder  = TimeEncoder(&timeEncoder{name: "millis", e: zapcore.EpochMillisTimeEncoder})
	nanosTimeEncoder   = TimeEncoder(&timeEncoder{name: "nanos", e: zapcore.EpochNanosTimeEncoder})
	secsTimeEncoder    = TimeEncoder(&timeEncoder{name: "secs", e: zapcore.EpochTimeEncoder})
	rfc3339TimeEncoder = TimeEncoder(&timeEncoder{
		name: "rfc3339",
		e: func(t time.Time, e zapcore.PrimitiveArrayEncoder) {
			encodeTimeLayout(t, "2006-01-02T15:04:05.000Z07:00", e)
		},
	})
)

func init() {
	must(RegisterTimeEncoder(iso8601TimeEncoder))
	must(RegisterTimeEncoder(millisTimeEncoder))
	must(RegisterTimeEncoder(nanosTimeEncoder))
	must(RegisterTimeEncoder(secsTimeEncoder))
	must(RegisterTimeEncoder(rfc3339TimeEncoder))
}

func encodeTimeLayout(t time.Time, layout string, e zapcore.PrimitiveArrayEncoder) {
	switch e := e.(type) {
	case interface{ AppendTimeLayout(time.Time, string) }:
		e.AppendTimeLayout(t, layout)
	default:
		e.AppendString(t.Format(layout))
	}
}

// ISO8601TimeEncoder serializes a time.Time to an ISO8601-formatted string with
// millisecond precision.
func ISO8601TimeEncoder() TimeEncoder { return iso8601TimeEncoder }

// RFC3339TimeEncoder serializes a time.Time to an RFC3339-formatted string with
// millisecond precision.
func RFC3339TimeEncoder() TimeEncoder { return rfc3339TimeEncoder }

// NanosecondsTimeEncoder serializes a time.Time to an integer number of nanoseconds
// since the Unix epoch.
func NanosecondsTimeEncoder() TimeEncoder { return nanosTimeEncoder }

// MillisecondsTimeEncoder serializes a time.Time to a floating-point number of
// milliseconds since the Unix epoch.
func MillisecondsTimeEncoder() TimeEncoder { return millisTimeEncoder }

// SecondsTimeEncoder serializes a time.Time to a floating-point number of seconds
// since the Unix epoch.
func SecondsTimeEncoder() TimeEncoder { return secsTimeEncoder }

type timeEncoderFlag struct {
	e *TimeEncoder
}

// TimeEncoderFlag returns a flag value for the encoder.
func TimeEncoderFlag(encoder *TimeEncoder) flag.Value {
	return &timeEncoderFlag{encoder}
}

func (f *timeEncoderFlag) Get() interface{} { return *f.e }
func (f *timeEncoderFlag) Set(s string) error {
	if e, ok := timeEncoders[s]; ok {
		*f.e = e
		return nil
	}
	return fmt.Errorf("zapr: unknown TimeEncoder: %q", s)
}
func (f *timeEncoderFlag) String() string {
	if f.e == nil {
		return ""
	}
	return (*f.e).Name()
}
