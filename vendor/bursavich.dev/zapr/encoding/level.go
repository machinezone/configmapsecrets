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

// A LevelEncoder provides a named zapcore.LevelEncoder.
type LevelEncoder interface {
	LevelEncoder() zapcore.LevelEncoder
	Name() string
}

var levelEncoders = make(map[string]LevelEncoder)

// RegisterLevelEncoder registers the LevelEncoder for use as a flag argument.
func RegisterLevelEncoder(e LevelEncoder) error {
	name := e.Name()
	if _, ok := levelEncoders[name]; ok {
		return fmt.Errorf("zapr: already registered LevelEncoder: %q", name)
	}
	levelEncoders[name] = e
	return nil
}

// LevelEncoders returns the registered LevelEncoders.
func LevelEncoders() []LevelEncoder {
	s := make([]LevelEncoder, 0, len(levelEncoders))
	for _, e := range levelEncoders {
		s = append(s, e)
	}
	sort.Slice(s, func(i, k int) bool { return s[i].Name() < s[k].Name() })
	return s
}

type levelEncoder struct {
	e    zapcore.LevelEncoder
	name string
}

func (e *levelEncoder) LevelEncoder() zapcore.LevelEncoder { return e.e }
func (e *levelEncoder) Name() string                       { return e.name }

var (
	colorLevelEncoder     = LevelEncoder(&levelEncoder{name: "color", e: zapcore.CapitalColorLevelEncoder})
	lowercaseLevelEncoder = LevelEncoder(&levelEncoder{name: "lower", e: zapcore.LowercaseLevelEncoder})
	uppercaseLevelEncoder = LevelEncoder(&levelEncoder{name: "upper", e: zapcore.CapitalLevelEncoder})
)

func init() {
	must(RegisterLevelEncoder(colorLevelEncoder))
	must(RegisterLevelEncoder(lowercaseLevelEncoder))
	must(RegisterLevelEncoder(uppercaseLevelEncoder))
}

// ColorLevelEncoder serializes a Level to an all-caps string and adds color.
// For example, InfoLevel is serialized to "INFO" and colored blue.
func ColorLevelEncoder() LevelEncoder { return colorLevelEncoder }

// LowercaseLevelEncoder serializes a Level to a lowercase string. For example,
// InfoLevel is serialized to "info".
func LowercaseLevelEncoder() LevelEncoder { return lowercaseLevelEncoder }

// UppercaseLevelEncoder serializes a Level to an all-caps string. For example,
// InfoLevel is serialized to "INFO".
func UppercaseLevelEncoder() LevelEncoder { return uppercaseLevelEncoder }

type levelEncoderFlag struct {
	e *LevelEncoder
}

// LevelEncoderFlag returns a flag value for the encoder.
func LevelEncoderFlag(encoder *LevelEncoder) flag.Value {
	return &levelEncoderFlag{encoder}
}

func (f *levelEncoderFlag) Get() interface{} { return *f.e }
func (f *levelEncoderFlag) Set(s string) error {
	if e, ok := levelEncoders[s]; ok {
		*f.e = e
		return nil
	}
	return fmt.Errorf("zapr: unknown LevelEncoder: %q", s)
}
func (f *levelEncoderFlag) String() string {
	if f.e == nil {
		return ""
	}
	return (*f.e).Name()
}
