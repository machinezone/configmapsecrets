// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright 2020 Andy Bursavich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package zapr provides a logr.Logger interface around a zap implementation,
// including metrics and a standard library log.Logger adapter.
package zapr

import (
	"bytes"
	"log"
	"reflect"

	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogSink represents the ability to log messages, both errors and not.
type LogSink interface {
	logr.LogSink
	logr.CallDepthLogSink

	// Underlying returns the underlying *zap.Logger with no caller skips.
	// Any names or added keys and values remain.
	Underlying() *zap.Logger

	// Flush writes any buffered data to the underlying io.Writer.
	Flush() error
}

type sink struct {
	logger   *zap.Logger
	depth    int
	errKey   string
	logLevel int
	maxLevel int
	observer Observer
}

// NewLogger returns a new Logger with the given options and a flush function.
func NewLogger(options ...Option) (logr.Logger, LogSink) {
	s := NewLogSink(options...)
	return logr.New(s), s
}

// NewLogSink returns a new LogSink with the given options.
func NewLogSink(options ...Option) LogSink {
	const depth = 1
	c := configWithOptions(options)
	return &sink{
		logger:   newLogger(c).WithOptions(zap.AddCallerSkip(depth)),
		errKey:   c.errorKey,
		depth:    depth,
		logLevel: 0,
		maxLevel: c.level,
		observer: c.observer,
	}
}

// newLogger returns a new zap.Logger with the given config.
func newLogger(c *config) *zap.Logger {
	var opts []zap.Option
	if c.development {
		opts = append(opts, zap.Development())
	}
	if c.enableCaller {
		opts = append(opts, zap.AddCaller())
	}
	if c.enableStacktrace {
		opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
	}
	if c.sampleFirst != 0 || c.sampleThereafter != 0 {
		opts = append(opts, zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return zapcore.NewSamplerWithOptions(core, c.sampleTick, c.sampleFirst, c.sampleThereafter, c.sampleOpts...)
		}))
	}
	enc := c.encoder.NewEncoder(zapcore.EncoderConfig{
		TimeKey:        c.timeKey,
		LevelKey:       c.levelKey,
		NameKey:        c.nameKey,
		CallerKey:      c.callerKey,
		FunctionKey:    c.functionKey,
		MessageKey:     c.messageKey,
		StacktraceKey:  c.stacktraceKey,
		LineEnding:     c.lineEnding,
		EncodeTime:     c.timeEncoder.TimeEncoder(),
		EncodeLevel:    c.levelEncoder.LevelEncoder(),
		EncodeDuration: c.durationEncoder.DurationEncoder(),
		EncodeCaller:   c.callerEncoder.CallerEncoder(),
	})
	if c.observer != nil {
		enc = &observerEncoder{
			Encoder:  enc,
			observer: c.observer,
		}
		c.observer.Init(c.name)
	}
	core := zapcore.NewCore(enc, c.ws, zapcore.InfoLevel)
	return zap.New(core, opts...).Named(c.name)
}

func (s *sink) sweeten(kvs []interface{}) []zapcore.Field {
	if len(kvs) == 0 {
		return nil
	}
	fields := make([]zapcore.Field, 0, len(kvs)/2)
	for i, n := 0, len(kvs)-1; i <= n; {
		switch key := kvs[i].(type) {
		case string:
			if i == n {
				s.sweetenDPanic("Ignored key without a value.",
					zap.Int("position", i),
					zap.String("key", key),
				)
				return fields
			}
			val := kvs[i+1]
			if x, ok := val.(logr.Marshaler); ok {
				val = x.MarshalLog()
			}
			fields = append(fields, zap.Any(key, val))
			i += 2
		case zapcore.Field:
			s.sweetenDPanic("Zap Field passed to logr",
				zap.Int("position", i),
				zap.String("key", key.Key),
			)
			fields = append(fields, key)
			i++
		default:
			s.sweetenDPanic("Ignored key-value pair with non-string key",
				zap.Int("position", i),
				zap.Any("type", reflect.TypeOf(key).String()),
			)
			i += 2
		}
	}
	return fields
}

func (s *sink) sweetenDPanic(msg string, fields ...zapcore.Field) {
	s.logger.WithOptions(zap.AddCallerSkip(1)).DPanic(msg, fields...)
}

func (s *sink) Init(info logr.RuntimeInfo) {
	s.logger = s.logger.WithOptions(zap.AddCallerSkip(info.CallDepth))
}

func (s *sink) Enabled(level int) bool { return level <= s.maxLevel }

func (s *sink) Info(level int, msg string, keysAndValues ...interface{}) {
	if level > s.maxLevel {
		return
	}
	if ce := s.logger.Check(zapcore.InfoLevel, msg); ce != nil {
		ce.Write(s.sweeten(keysAndValues)...)
	}
}

func (s *sink) Error(err error, msg string, keysAndValues ...interface{}) {
	if ce := s.logger.Check(zapcore.ErrorLevel, msg); ce != nil {
		kvs := keysAndValues
		if s.errKey != "" && err != nil {
			kvs = make([]interface{}, 0, len(keysAndValues)+2)
			kvs = append(kvs, keysAndValues...)
			kvs = append(kvs, s.errKey, err.Error())
		}
		ce.Write(s.sweeten(kvs)...)
	}
}

func (s *sink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	v := *s
	v.logger = s.logger.With(s.sweeten(keysAndValues)...)
	return &v
}

func (s *sink) WithName(name string) logr.LogSink {
	v := *s
	v.logger = v.logger.Named(name)
	if v.observer != nil {
		v.observer.Init(loggerName(v.logger))
	}
	return &v
}

func (s *sink) WithCallDepth(depth int) logr.LogSink {
	if depth == 0 {
		return s
	}
	v := *s
	v.logger = v.logger.WithOptions(zap.AddCallerSkip(depth))
	v.depth += depth
	return &v
}

func (s *sink) Underlying() *zap.Logger {
	return s.logger.WithOptions(zap.AddCallerSkip(-s.depth))
}

func (s *sink) Flush() error { return s.logger.Sync() }

var runtimeInfo logr.RuntimeInfo

func init() {
	logr.New((*infoSink)(&runtimeInfo))
}

type infoSink logr.RuntimeInfo

func (s *infoSink) Init(info logr.RuntimeInfo)                              { *s = (infoSink)(info) }
func (s *infoSink) WithValues(keysAndValues ...interface{}) logr.LogSink    { return s }
func (s *infoSink) WithName(name string) logr.LogSink                       { return s }
func (*infoSink) Enabled(level int) bool                                    { return false }
func (*infoSink) Info(level int, msg string, keysAndValues ...interface{})  {}
func (*infoSink) Error(err error, msg string, keysAndValues ...interface{}) {}

// NewStdInfoLogger returns a *log.Logger which writes to the supplied Logger's Info method.
func NewStdInfoLogger(s logr.CallDepthLogSink) *log.Logger {
	infoFn := s.WithCallDepth(4 - runtimeInfo.CallDepth).Info
	fn := func(msg string, _ ...interface{}) { infoFn(0, msg) }
	return log.New(stdLogWriterFunc(fn), "" /*prefix*/, 0 /*flags*/)
}

// NewStdErrorLogger returns a *log.Logger which writes to the supplied Logger's Error method.
func NewStdErrorLogger(s logr.CallDepthLogSink) *log.Logger {
	errFn := s.WithCallDepth(4 - runtimeInfo.CallDepth).Error
	fn := func(msg string, _ ...interface{}) { errFn(nil, msg) }
	return log.New(stdLogWriterFunc(fn), "" /*prefix*/, 0 /*flags*/)
}

type stdLogWriterFunc func(msg string, _ ...interface{})

func (fn stdLogWriterFunc) Write(b []byte) (int, error) {
	v := bytes.TrimSpace(b)
	fn(string(v))
	return len(b), nil
}
