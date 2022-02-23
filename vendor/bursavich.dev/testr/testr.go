// SPDX-License-Identifier: MIT
//
// Copyright 2022 Andrew Bursavich. All rights reserved.
// Use of this source code is governed by The MIT License
// which can be found in the LICENSE file.

// Package testr provides a logr.LogSink implementation using *testing.T.
package testr

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
)

var (
	_ logr.LogSink                = &sink{}
	_ logr.CallStackHelperLogSink = &sink{}
)

// An Option sets an option.
type Option func(*option)

type option struct {
	format funcr.Options
	errors bool
}

// WithErrors returns an Option that enables mapping error logs to testing errors.
func WithErrors(enabled bool) Option {
	return func(o *option) { o.errors = enabled }
}

// WithVerbosity returns an Option that sets the verbosity.
func WithVerbosity(verbosity int) Option {
	return func(o *option) { o.format.Verbosity = verbosity }
}

// NewLogger returns a logr.Logger that outputs through t with the given options.
func NewLogger(t *testing.T, options ...Option) logr.Logger {
	var opt option
	for _, fn := range options {
		fn(&opt)
	}
	return logr.New(&sink{
		t:      t,
		fmt:    funcr.NewFormatter(opt.format),
		errors: opt.errors,
	})
}

type sink struct {
	t      *testing.T
	fmt    funcr.Formatter
	errors bool
}

func (s *sink) Init(info logr.RuntimeInfo) {
	s.fmt.Init(info)
}

func (s *sink) Enabled(level int) bool {
	return s.fmt.Enabled(level)
}

func (s *sink) Info(level int, msg string, kvList ...interface{}) {
	s.t.Helper()
	s.t.Log(format(s.fmt.FormatInfo(level, msg, kvList)))
}

func (s *sink) Error(err error, msg string, kvList ...interface{}) {
	s.t.Helper()
	if v := format(s.fmt.FormatError(err, msg, kvList)); s.errors {
		s.t.Error(v)
	} else {
		s.t.Log(v)
	}
}

func (s sink) WithName(name string) logr.LogSink {
	s.fmt.AddName(name)
	return &s
}

func (s sink) WithValues(kvList ...interface{}) logr.LogSink {
	s.fmt.AddValues(kvList)
	return &s
}

func (s *sink) GetCallStackHelper() func() {
	return s.t.Helper
}

func format(prefix, args string) string {
	if prefix != "" {
		return prefix + ": " + args
	}
	return args
}
