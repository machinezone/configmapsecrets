// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright 2020 Andy Bursavich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package encoding provides named encoders with flag integration.
package encoding

import (
	"flag"
	"fmt"
	"sort"

	"go.uber.org/zap/zapcore"
)

// An Encoder provides a named zapcore.Encoder.
type Encoder interface {
	NewEncoder(zapcore.EncoderConfig) zapcore.Encoder
	Name() string
}

var encoders = make(map[string]Encoder)

// RegisterEncoder registers the Encoder for use as a flag argument.
func RegisterEncoder(e Encoder) error {
	name := e.Name()
	if _, ok := encoders[name]; ok {
		return fmt.Errorf("zapr: already registered Encoder: %q", name)
	}
	encoders[name] = e
	return nil
}

// Encoders returns the registered Encoders.
func Encoders() []Encoder {
	s := make([]Encoder, 0, len(encoders))
	for _, e := range encoders {
		s = append(s, e)
	}
	sort.Slice(s, func(i, k int) bool { return s[i].Name() < s[k].Name() })
	return s
}

type encoder struct {
	ctor func(zapcore.EncoderConfig) zapcore.Encoder
	name string
}

func (e *encoder) NewEncoder(c zapcore.EncoderConfig) zapcore.Encoder { return e.ctor(c) }
func (e *encoder) Name() string                                       { return e.name }

var (
	consoleEncoder = Encoder(&encoder{name: "console", ctor: zapcore.NewConsoleEncoder})
	jsonEncoder    = Encoder(&encoder{name: "json", ctor: zapcore.NewJSONEncoder})
)

func init() {
	must(RegisterEncoder(consoleEncoder))
	must(RegisterEncoder(jsonEncoder))
}

// ConsoleEncoder creates an encoder whose output is designed for human
// consumption, rather than machine consumption.
func ConsoleEncoder() Encoder { return consoleEncoder }

// JSONEncoder creates a fast, low-allocation JSON encoder.
func JSONEncoder() Encoder { return jsonEncoder }

type encoderFlag struct{ e *Encoder }

// EncoderFlag returns a flag value for the encoder.
func EncoderFlag(encoder *Encoder) flag.Value {
	return &encoderFlag{encoder}
}

func (f *encoderFlag) Get() interface{} { return *f.e }
func (f *encoderFlag) Set(s string) error {
	if e, ok := encoders[s]; ok {
		*f.e = e
		return nil
	}
	return fmt.Errorf("zapr: unknown Encoder: %q", s)
}
func (f *encoderFlag) String() string {
	if f.e == nil {
		return ""
	}
	return (*f.e).Name()
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
