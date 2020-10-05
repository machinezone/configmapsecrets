// Copyright 2020 Andy Bursavich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package zapr provides a logr.Logger interface around a zap implementation,
// with Prometheus metrics and a standard library log.Logger adapter.
package zapr

import (
	"bytes"
	"log"
	"reflect"

	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger represents the ability to log messages, both errors and not.
type Logger interface {
	// Logger is the main logging interface. All methods that return a logr.Logger
	// (e.g. V, WithValues, WithName) will return a value implementing this Logger
	// interface (e.g. with the additional Underlying and Flush methods).
	logr.Logger

	// Underlying returns the underlying *zap.Logger with no extra
	// caller skips. It may return nil if the logger is disabled.
	Underlying() *zap.Logger

	// Flush writes any buffered data to the underlying io.Writer.
	Flush() error
}

var noop = Logger(noopLogger{})

// Noop returns a disabled Logger for which all operations are no-op.
func Noop() Logger { return noop }

type noopLogger struct{}

func (noopLogger) Enabled() bool                                             { return false }
func (noopLogger) Info(msg string, keysAndValues ...interface{})             {}
func (noopLogger) Error(err error, msg string, keysAndValues ...interface{}) {}
func (noopLogger) V(level int) logr.Logger                                   { return noop }
func (noopLogger) WithValues(keysAndValues ...interface{}) logr.Logger       { return noop }
func (noopLogger) WithName(name string) logr.Logger                          { return noop }
func (noopLogger) Underlying() *zap.Logger                                   { return nil }
func (noopLogger) Flush() error                                              { return nil }

type zapLogr struct {
	underlying *zap.Logger
	logger     *zap.Logger
	errKey     string
	logLevel   int
	maxLevel   int
	metrics    Metrics
}

// NewLogger creates a new logr.Logger with the given Config.
func NewLogger(cfg *Config) Logger {
	underlying := newZapLogger(cfg)
	if cfg.Metrics != nil {
		cfg.Metrics.Init(loggerName(underlying))
	}
	logger := underlying.WithOptions(zap.AddCallerSkip(1))
	return &zapLogr{
		underlying: underlying,
		logger:     logger,
		errKey:     cfg.ErrorKey,
		logLevel:   0,
		maxLevel:   cfg.Level,
		metrics:    cfg.Metrics,
	}
}

func (z *zapLogr) sweeten(kvs []interface{}) []zapcore.Field {
	if len(kvs) == 0 {
		return nil
	}
	fields := make([]zapcore.Field, 0, len(kvs)/2)
	for i, n := 0, len(kvs)-1; i <= n; {
		switch key := kvs[i].(type) {
		case string:
			if i == n {
				z.sweetenDPanic("Ignored key without a value.",
					zap.Int("position", i),
					zap.String("key", key),
				)
				return fields
			}
			fields = append(fields, zap.Any(key, kvs[i+1]))
			i += 2
		case zapcore.Field:
			z.sweetenDPanic("Zap Field passed to logr",
				zap.Int("position", i),
				zap.String("key", key.Key),
			)
			fields = append(fields, key)
			i++
		default:
			z.sweetenDPanic("Ignored key-value pair with non-string key",
				zap.Int("position", i),
				zap.Any("type", reflect.TypeOf(key).String()),
			)
			i += 2
		}
	}
	return fields
}

func (z *zapLogr) sweetenDPanic(msg string, fields ...zapcore.Field) {
	z.logger.WithOptions(zap.AddCallerSkip(2)).DPanic(msg, fields...)
}

func (z *zapLogr) Enabled() bool { return true }

func (z *zapLogr) Info(msg string, keysAndValues ...interface{}) {
	if ce := z.logger.Check(zapcore.InfoLevel, msg); ce != nil {
		ce.Write(z.sweeten(keysAndValues)...)
	}
}

func (z *zapLogr) Error(err error, msg string, keysAndValues ...interface{}) {
	if ce := z.logger.Check(zapcore.ErrorLevel, msg); ce != nil {
		kvs := keysAndValues
		if z.errKey != "" && err != nil {
			kvs = make([]interface{}, 0, len(keysAndValues)+2)
			kvs = append(kvs, keysAndValues...)
			kvs = append(kvs, z.errKey, err.Error())
		}
		ce.Write(z.sweeten(kvs)...)
	}
}

func (z *zapLogr) V(level int) logr.Logger {
	switch next := z.logLevel + level; {
	case level == 0:
		return z
	case level > 0 && next <= z.maxLevel:
		v := *z
		v.logLevel = next
		return &v
	default:
		return noop
	}
}

func (z *zapLogr) WithValues(keysAndValues ...interface{}) logr.Logger {
	v := *z
	v.logger = z.logger.With(z.sweeten(keysAndValues)...)
	return &v
}

func (z *zapLogr) WithName(name string) logr.Logger {
	v := *z
	v.underlying = v.underlying.Named(name)
	v.logger = v.underlying.WithOptions(zap.AddCallerSkip(1))
	if v.metrics != nil {
		v.metrics.Init(loggerName(v.underlying))
	}
	return &v
}

func (z *zapLogr) Underlying() *zap.Logger { return z.underlying }

func (z *zapLogr) Flush() error { return z.logger.Sync() }

// NewStdInfoLogger returns a *log.Logger which writes to the supplied Logger's Info method.
func NewStdInfoLogger(logger Logger) *log.Logger {
	if !logger.Enabled() {
		return newNoopStdLogWriter()
	}
	fn := logger.Underlying().WithOptions(zap.AddCallerSkip(3)).Info
	return log.New(stdLogWriterFunc(fn), "" /*prefix*/, 0 /*flags*/)
}

// NewStdErrorLogger returns a *log.Logger which writes to the supplied Logger's Error method.
func NewStdErrorLogger(logger Logger) *log.Logger {
	if !logger.Enabled() {
		return newNoopStdLogWriter()
	}
	fn := logger.Underlying().WithOptions(zap.AddCallerSkip(3)).Error
	return log.New(stdLogWriterFunc(fn), "" /*prefix*/, 0 /*flags*/)
}

type stdLogWriterFunc func(msg string, fields ...zap.Field)

func (fn stdLogWriterFunc) Write(b []byte) (int, error) {
	v := bytes.TrimSpace(b)
	fn(string(v))
	return len(b), nil
}

var noopWriter noopStdLogWriter

func newNoopStdLogWriter() *log.Logger {
	return log.New(noopWriter, "" /*prefix*/, 0 /*flags*/)
}

type noopStdLogWriter struct{}

func (noopStdLogWriter) Write(b []byte) (int, error) {
	return len(b), nil
}
