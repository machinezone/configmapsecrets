// Copyright 2020 Andy Bursavich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package zapr

import (
	"flag"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"bursavich.dev/zapr/encoding"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	defaultEncoder         = encoding.JSONEncoder()
	defaultTimeEncoder     = encoding.ISO8601TimeEncoder()
	defaultLevelEncoder    = encoding.UppercaseLevelEncoder()
	defaultDurationEncoder = encoding.SecondsDurationEncoder()
	defaultCallerEncoder   = encoding.ShortCallerEncoder()
)

// newZapLogger returns a new zap.Logger with the given config.
func newZapLogger(c *Config) *zap.Logger {
	var opts []zap.Option
	if c.Development {
		opts = append(opts, zap.Development())
	}
	if c.EnableCaller {
		opts = append(opts, zap.AddCaller())
	}
	if c.EnableStacktrace {
		opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
	}
	if c.SampleInitial != 0 || c.SampleThereafter != 0 {
		opts = append(opts, zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return zapcore.NewSamplerWithOptions(core, time.Second, c.SampleInitial, c.SampleThereafter)
		}))
	}
	core := zapcore.NewCore(
		c.encoder(),
		zapcore.Lock(os.Stderr),
		zapcore.InfoLevel,
	)
	return zap.New(core, opts...).Named(c.Name)
}

// Config specifies the configuration of a Logger.
type Config struct {
	Name  string
	Level int

	TimeKey       string
	LevelKey      string
	NameKey       string
	CallerKey     string
	FunctionKey   string
	MessageKey    string
	ErrorKey      string
	StacktraceKey string
	LineEnding    string

	Encoder         encoding.Encoder
	TimeEncoder     encoding.TimeEncoder
	LevelEncoder    encoding.LevelEncoder
	DurationEncoder encoding.DurationEncoder
	CallerEncoder   encoding.CallerEncoder

	EnableStacktrace bool
	EnableCaller     bool
	Development      bool

	SampleInitial    int
	SampleThereafter int

	Metrics Metrics
}

// DefaultConfig returns the default Config.
func DefaultConfig() *Config {
	return &Config{
		Name:             "",
		Level:            0,
		TimeKey:          "time",
		LevelKey:         "level",
		NameKey:          "logger",
		CallerKey:        "caller",
		FunctionKey:      "",
		MessageKey:       "msg",
		ErrorKey:         "error",
		StacktraceKey:    "stacktrace",
		LineEnding:       zapcore.DefaultLineEnding,
		Encoder:          defaultEncoder,
		TimeEncoder:      defaultTimeEncoder,
		LevelEncoder:     defaultLevelEncoder,
		DurationEncoder:  defaultDurationEncoder,
		CallerEncoder:    defaultCallerEncoder,
		EnableStacktrace: false,
		EnableCaller:     true,
		Development:      false,
		SampleInitial:    100,
		SampleThereafter: 100,
		Metrics:          nil,
	}
}

// DevelopmentConfig returns a development-friendly Config.
func DevelopmentConfig() *Config {
	cfg := DefaultConfig()
	cfg.Level = 2
	cfg.FunctionKey = "func"
	cfg.Encoder = encoding.ConsoleEncoder()
	cfg.LevelEncoder = encoding.ColorLevelEncoder()
	cfg.DurationEncoder = encoding.StringDurationEncoder()
	cfg.EnableStacktrace = true
	cfg.Development = true
	return cfg
}

func (c *Config) encoder() zapcore.Encoder {
	enc := c.newEncoder(zapcore.EncoderConfig{
		TimeKey:        c.TimeKey,
		LevelKey:       c.LevelKey,
		NameKey:        c.NameKey,
		CallerKey:      c.CallerKey,
		FunctionKey:    c.FunctionKey,
		MessageKey:     c.MessageKey,
		StacktraceKey:  c.StacktraceKey,
		LineEnding:     c.LineEnding,
		EncodeTime:     c.timeEncoder(),
		EncodeLevel:    c.levelEncoder(),
		EncodeDuration: c.durationEncoder(),
		EncodeCaller:   c.callerEncoder(),
	})
	if c.Metrics != nil {
		return &metricsEncoder{
			Encoder: enc,
			metrics: c.Metrics,
		}
	}
	return enc
}

func (c *Config) newEncoder(cfg zapcore.EncoderConfig) zapcore.Encoder {
	if c == nil || c.Encoder == nil {
		return defaultEncoder.NewEncoder(cfg)
	}
	return c.Encoder.NewEncoder(cfg)
}

func (c *Config) timeEncoder() zapcore.TimeEncoder {
	if c == nil || c.TimeEncoder == nil {
		return defaultTimeEncoder.TimeEncoder()
	}
	return c.TimeEncoder.TimeEncoder()
}

func (c *Config) levelEncoder() zapcore.LevelEncoder {
	if c == nil || c.LevelEncoder == nil {
		return defaultLevelEncoder.LevelEncoder()
	}
	return c.LevelEncoder.LevelEncoder()
}

func (c *Config) durationEncoder() zapcore.DurationEncoder {
	if c == nil || c.DurationEncoder == nil {
		return defaultDurationEncoder.DurationEncoder()
	}
	return c.DurationEncoder.DurationEncoder()
}

func (c *Config) callerEncoder() zapcore.CallerEncoder {
	if c == nil || c.CallerEncoder == nil {
		return defaultCallerEncoder.CallerEncoder()
	}
	return c.CallerEncoder.CallerEncoder()
}

// RegisterCommonFlags registers basic fields of the Config as flags in the
// FlagSet. If fs is nil, flag.CommandLine is used.
func (c *Config) RegisterCommonFlags(fs *flag.FlagSet) *Config {
	if fs == nil {
		fs = flag.CommandLine
	}
	fs.IntVar(&c.Level, "log-level", c.Level, "Log level.")
	c.registerEncoderFlag(fs)
	c.registerTimeEncoderFlag(fs)
	c.registerLevelEncoderFlag(fs)
	c.registerCallerEncoderFlag(fs)
	return c
}

// RegisterFlags registers fields of the Config as flags in the FlagSet.
// If fs is nil, flag.CommandLine is used.
func (c *Config) RegisterFlags(fs *flag.FlagSet) *Config {
	if fs == nil {
		fs = flag.CommandLine
	}
	c.RegisterCommonFlags(fs)
	c.registerDurationEncoderFlag(fs)
	fs.StringVar(&c.TimeKey, "log-time-key", c.TimeKey, "Log time key.")
	fs.StringVar(&c.LevelKey, "log-level-key", c.LevelKey, "Log level key.")
	fs.StringVar(&c.MessageKey, "log-message-key", c.MessageKey, "Log message key.")
	fs.StringVar(&c.CallerKey, "log-caller-key", c.CallerKey, "Log caller key.")
	fs.StringVar(&c.FunctionKey, "log-function-key", c.FunctionKey, "Log function key.")
	fs.StringVar(&c.StacktraceKey, "log-stacktrace-key", c.StacktraceKey, "Log stacktrace key.")
	fs.BoolVar(&c.EnableStacktrace, "log-stacktrace", c.EnableStacktrace, `Log stacktrace on error.`)
	fs.BoolVar(&c.EnableCaller, "log-caller", c.EnableCaller, `Log caller file and line.`)
	fs.IntVar(&c.SampleInitial, "log-sample-initial", c.SampleInitial, "Log every call up to this count per second.")
	fs.IntVar(&c.SampleThereafter, "log-sample-thereafter", c.SampleThereafter, "Log only one of this many calls after reaching the initial sample per second.")
	return c
}

func (c *Config) registerEncoderFlag(fs *flag.FlagSet) {
	var names []string
	for _, e := range encoding.Encoders() {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	fs.Var(encoding.EncoderFlag(&c.Encoder), "log-format", `Log format (e.g. `+listNames(names)+`).`)
}

func (c *Config) registerTimeEncoderFlag(fs *flag.FlagSet) {
	var names []string
	for _, e := range encoding.TimeEncoders() {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	fs.Var(encoding.TimeEncoderFlag(&c.TimeEncoder), "log-time-format", `Log time format (e.g. `+listNames(names)+`).`)
}

func (c *Config) registerLevelEncoderFlag(fs *flag.FlagSet) {
	var names []string
	for _, e := range encoding.LevelEncoders() {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	fs.Var(encoding.LevelEncoderFlag(&c.LevelEncoder), "log-level-format", `Log level format (e.g. `+listNames(names)+`).`)
}

func (c *Config) registerCallerEncoderFlag(fs *flag.FlagSet) {
	var names []string
	for _, e := range encoding.CallerEncoders() {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	fs.Var(encoding.CallerEncoderFlag(&c.CallerEncoder), "log-caller-format", `Log caller format (e.g. `+listNames(names)+`).`)
}

func (c *Config) registerDurationEncoderFlag(fs *flag.FlagSet) {
	var names []string
	for _, e := range encoding.DurationEncoders() {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	fs.Var(encoding.DurationEncoderFlag(&c.DurationEncoder), "log-duration-format", `Log duration format (e.g. `+listNames(names)+`).`)
}

func listNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return strconv.Quote(names[0])
	case 2:
		return strconv.Quote(names[0]) + " or " + strconv.Quote(names[1])
	default:
		var b strings.Builder
		last := len(names) - 1
		for _, name := range names[:last] {
			b.WriteString(strconv.Quote(name))
			b.WriteString(", ")
		}
		b.WriteString("or ")
		b.WriteString(strconv.Quote(names[last]))
		return b.String()
	}
}
