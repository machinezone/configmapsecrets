// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright 2020 Andy Bursavich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package zapr

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"bursavich.dev/zapr/encoding"
	"go.uber.org/zap/zapcore"
)

type config struct {
	ws    zapcore.WriteSyncer
	name  string
	level int

	timeKey       string
	levelKey      string
	nameKey       string
	callerKey     string
	functionKey   string
	messageKey    string
	errorKey      string
	stacktraceKey string
	lineEnding    string

	encoder         encoding.Encoder
	timeEncoder     encoding.TimeEncoder
	levelEncoder    encoding.LevelEncoder
	durationEncoder encoding.DurationEncoder
	callerEncoder   encoding.CallerEncoder

	enableStacktrace bool
	enableCaller     bool
	development      bool

	sampleTick       time.Duration
	sampleFirst      int
	sampleThereafter int
	sampleOpts       []zapcore.SamplerOption

	observer Observer
}

func configWithOptions(options []Option) *config {
	c := &config{
		ws:               stderr(),
		name:             "",
		level:            0,
		timeKey:          "time",
		levelKey:         "level",
		nameKey:          "logger",
		callerKey:        "caller",
		functionKey:      "",
		messageKey:       "message",
		errorKey:         "error",
		stacktraceKey:    "stacktrace",
		lineEnding:       zapcore.DefaultLineEnding,
		encoder:          encoding.JSONEncoder(),
		timeEncoder:      encoding.ISO8601TimeEncoder(),
		levelEncoder:     encoding.UppercaseLevelEncoder(),
		durationEncoder:  encoding.SecondsDurationEncoder(),
		callerEncoder:    encoding.ShortCallerEncoder(),
		enableStacktrace: false,
		enableCaller:     true,
		development:      false,
		sampleTick:       time.Second,
		sampleFirst:      100,
		sampleThereafter: 100,
		observer:         nil,
	}
	for _, o := range sortedOptions(options) {
		o.apply(c)
	}
	return c
}

func stderr() zapcore.WriteSyncer {
	if err := os.Stderr.Sync(); err != nil {
		// TODO: errors.Is(syscall.EINVAL)
		return &stderrNoopSyncer{}
	}
	return zapcore.Lock(os.Stderr)
}

type stderrNoopSyncer struct {
	mu sync.Mutex
}

func (s *stderrNoopSyncer) Write(b []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.Stderr.Write(b)
}

func (*stderrNoopSyncer) Sync() error { return nil }

// An Option applies optional configuration.
type Option interface {
	apply(*config)
	register(*flag.FlagSet)
	weight() int
}

type opt struct {
	applyFn    func(*config)
	registerFn func(*flag.FlagSet)
	wgt        int
}

func (o opt) apply(c *config)          { o.applyFn(c) }
func (o opt) register(f *flag.FlagSet) { o.registerFn(f) }
func (o opt) weight() int              { return o.wgt }

func optionFunc(fn func(*config)) Option {
	return opt{
		applyFn:    fn,
		registerFn: func(*flag.FlagSet) {},
	}
}

type byWeightDesc []Option

func (s byWeightDesc) Len() int           { return len(s) }
func (s byWeightDesc) Less(i, k int) bool { return s[i].weight() > s[k].weight() } // reversed
func (s byWeightDesc) Swap(i, k int)      { s[i], s[k] = s[k], s[i] }

func sortedOptions(options []Option) []Option {
	if sort.IsSorted(byWeightDesc(options)) {
		return options
	}
	// sort a copy
	options = append(make([]Option, 0, len(options)), options...)
	sort.Stable(byWeightDesc(options))
	return options
}

// WithWriteSyncer returns an Option that sets the underlying writer.
// The default value is stderr.
func WithWriteSyncer(ws zapcore.WriteSyncer) Option {
	return opt{
		applyFn:    func(c *config) { c.ws = ws },
		registerFn: func(fs *flag.FlagSet) {},
	}
}

// WithObserver returns an Option that sets the metrics Observer.
// There is no default Observer.
func WithObserver(observer Observer) Option {
	return opt{
		applyFn:    func(c *config) { c.observer = observer },
		registerFn: func(fs *flag.FlagSet) {},
	}
}

// WithName returns an Option that sets the name.
// The default value is empty.
func WithName(name string) Option {
	return opt{
		applyFn: func(c *config) { c.name = name },
		registerFn: func(fs *flag.FlagSet) {
			fs.StringVar(&name, "log-name", name, "Log name.")
		},
	}
}

// WithLevel returns an Option that sets the level.
// The default value is 0.
func WithLevel(level int) Option {
	return opt{
		applyFn: func(c *config) { c.level = level },
		registerFn: func(fs *flag.FlagSet) {
			fs.IntVar(&level, "log-level", level, "Log verbosity level.")
		},
	}
}

// WithTimeKey returns an Option that sets the time key.
// The default value is "time".
func WithTimeKey(key string) Option {
	return opt{
		applyFn: func(c *config) { c.timeKey = key },
		registerFn: func(fs *flag.FlagSet) {
			fs.StringVar(&key, "log-time-key", key, "Log time key.")
		},
	}
}

// WithLevelKey returns an Option that sets the level key.
// The default value is "level".
func WithLevelKey(key string) Option {
	return opt{
		applyFn: func(c *config) { c.levelKey = key },
		registerFn: func(fs *flag.FlagSet) {
			fs.StringVar(&key, "log-level-key", key, "Log level key.")
		},
	}
}

// WithNameKey returns an Option that sets the name key.
// The default value is "logger".
func WithNameKey(key string) Option {
	return opt{
		applyFn: func(c *config) { c.nameKey = key },
		registerFn: func(fs *flag.FlagSet) {
			fs.StringVar(&key, "log-name-key", key, "Log name key.")
		},
	}
}

// WithCallerKey returns an Option that sets the caller key.
// The default value is "caller".
func WithCallerKey(key string) Option {
	return opt{
		applyFn: func(c *config) { c.callerKey = key },
		registerFn: func(fs *flag.FlagSet) {
			fs.StringVar(&key, "log-caller-key", key, "Log caller key.")
		},
	}
}

// WithFunctionKey returns an Option that sets the function key.
// The default value is empty.
func WithFunctionKey(key string) Option {
	return opt{
		applyFn: func(c *config) { c.functionKey = key },
		registerFn: func(fs *flag.FlagSet) {
			fs.StringVar(&key, "log-function-key", key, "Log function key.")
		},
	}
}

// WithMessageKey returns an Option that sets the message key.
// The default value is "message".
func WithMessageKey(key string) Option {
	return opt{
		applyFn: func(c *config) { c.messageKey = key },
		registerFn: func(fs *flag.FlagSet) {
			fs.StringVar(&key, "log-message-key", key, "Log message key.")
		},
	}
}

// WithErrorKey returns an Option that sets the error key.
// The default value is "error".
func WithErrorKey(key string) Option {
	return opt{
		applyFn: func(c *config) { c.errorKey = key },
		registerFn: func(fs *flag.FlagSet) {
			fs.StringVar(&key, "log-error-key", key, "Log error key.")
		},
	}
}

// WithStacktraceKey returns an Option that sets the stacktrace key.
// The default value is "stacktrace".
func WithStacktraceKey(key string) Option {
	return opt{
		applyFn: func(c *config) { c.stacktraceKey = key },
		registerFn: func(fs *flag.FlagSet) {
			fs.StringVar(&key, "log-stacktrace-key", key, "Log stacktrace key.")
		},
	}
}

// WithLineEnding returns an Option that sets the line-ending.
// The default value is "\n".
func WithLineEnding(ending string) Option {
	return opt{
		applyFn: func(c *config) { c.lineEnding = ending },
		registerFn: func(fs *flag.FlagSet) {
			fs.StringVar(&ending, "log-line-ending", ending, "Log line ending.")
		},
	}
}

// WithEncoder returns an Option that sets the encoder.
// The default value is a JSONEncoder.
func WithEncoder(encoder encoding.Encoder) Option {
	var names []string
	for _, e := range encoding.Encoders() {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	usage := fmt.Sprintf("Log format (e.g. %s).", listNames(names))

	return opt{
		applyFn: func(c *config) { c.encoder = encoder },
		registerFn: func(fs *flag.FlagSet) {
			fs.Var(encoding.EncoderFlag(&encoder), "log-format", usage)
		},
	}
}

// WithTimeEncoder returns an Option that sets the encoder.
// The default encoding is ISO 8601.
func WithTimeEncoder(encoder encoding.TimeEncoder) Option {
	var names []string
	for _, e := range encoding.TimeEncoders() {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	usage := fmt.Sprintf("Log time format (e.g. %s).", listNames(names))

	return opt{
		applyFn: func(c *config) { c.timeEncoder = encoder },
		registerFn: func(fs *flag.FlagSet) {
			fs.Var(encoding.TimeEncoderFlag(&encoder), "log-time-format", usage)
		},
	}
}

// WithLevelEncoder returns an Option that sets the level encoder.
// The default encoding is uppercase.
func WithLevelEncoder(encoder encoding.LevelEncoder) Option {
	var names []string
	for _, e := range encoding.LevelEncoders() {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	usage := fmt.Sprintf("Log level format (e.g. %s).", listNames(names))

	return opt{
		applyFn: func(c *config) { c.levelEncoder = encoder },
		registerFn: func(fs *flag.FlagSet) {
			fs.Var(encoding.LevelEncoderFlag(&encoder), "log-level-format", usage)
		},
	}
}

// WithDurationEncoder returns an Option that sets the duration encoder.
// The default encoding is seconds.
func WithDurationEncoder(encoder encoding.DurationEncoder) Option {
	var names []string
	for _, e := range encoding.DurationEncoders() {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	usage := fmt.Sprintf("Log duration format (e.g. %s).", listNames(names))

	return opt{
		applyFn: func(c *config) { c.durationEncoder = encoder },
		registerFn: func(fs *flag.FlagSet) {
			fs.Var(encoding.DurationEncoderFlag(&encoder), "log-duration-format", usage)
		},
	}
}

// WithCallerEncoder returns an Option that sets the caller encoder.
// The default encoding is short.
func WithCallerEncoder(encoder encoding.CallerEncoder) Option {
	var names []string
	for _, e := range encoding.CallerEncoders() {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	usage := fmt.Sprintf("Log caller format (e.g. %s).", listNames(names))

	return opt{
		applyFn: func(c *config) { c.callerEncoder = encoder },
		registerFn: func(fs *flag.FlagSet) {
			fs.Var(encoding.CallerEncoderFlag(&encoder), "log-caller-format", usage)
		},
	}
}

// WithCallerEnabled returns an Option that sets whether the caller field
// is enabled. It's enabled by default.
func WithCallerEnabled(enabled bool) Option {
	return opt{
		applyFn: func(c *config) { c.enableCaller = enabled },
		registerFn: func(fs *flag.FlagSet) {
			fs.BoolVar(&enabled, "log-caller", enabled, "Log caller file and line.")
		},
	}
}

// WithStacktraceEnabled returns an Option that sets whether the stacktrace
// field is enabled. It's disabled by default.
func WithStacktraceEnabled(enabled bool) Option {
	return opt{
		applyFn: func(c *config) { c.enableStacktrace = enabled },
		registerFn: func(fs *flag.FlagSet) {
			fs.BoolVar(&enabled, "log-stacktrace", enabled, "Log stacktrace on error.")
		},
	}
}

// WithSampler returns an Option that sets sampler options.
// The default is 1s tick, 100 first, and 100 thereafter.
func WithSampler(tick time.Duration, first, thereafter int, opts ...zapcore.SamplerOption) Option {
	return opt{
		applyFn: func(c *config) {
			c.sampleTick = tick
			c.sampleFirst = first
			c.sampleThereafter = thereafter
			c.sampleOpts = opts
		},
		registerFn: func(fs *flag.FlagSet) {
			fs.DurationVar(&tick, "log-sampler-tick", tick, "Sample logs over this duration.")
			fs.IntVar(&first, "log-sampler-first", first, "Log every call up to this count per tick.")
			fs.IntVar(&thereafter, "log-sampler-thereafter", thereafter, "Log only one of this many calls after reaching the first sample per tick.")
		},
	}
}

// WithDevelopmentOptions returns an Option that enables a set of
// development-friendly options.
func WithDevelopmentOptions(enabled bool) Option {
	return opt{
		applyFn: func(c *config) {
			if !enabled {
				return
			}
			c.level = 3
			c.functionKey = "func"
			c.encoder = encoding.ConsoleEncoder()
			c.levelEncoder = encoding.ColorLevelEncoder()
			c.durationEncoder = encoding.StringDurationEncoder()
			c.enableStacktrace = true
			c.development = true
		},
		registerFn: func(fs *flag.FlagSet) {
			fs.BoolVar(&enabled, "log-development", enabled, "Log with development-friendly defaults.")
		},
		wgt: 1,
	}
}

// RegisterFlags registers the given Options with the FlagSet.
func RegisterFlags(fs *flag.FlagSet, options ...Option) []Option {
	if fs == nil {
		fs = flag.CommandLine
	}
	for _, o := range options {
		o.register(fs)
	}
	return options
}

// AllOptions returns all Options with the given overrides.
func AllOptions(overrides ...Option) []Option {
	c := configWithOptions(overrides)
	return []Option{
		WithWriteSyncer(c.ws),
		WithObserver(c.observer),
		WithName(c.name),
		WithLevel(c.level),
		WithTimeKey(c.timeKey),
		WithLevelKey(c.levelKey),
		WithNameKey(c.nameKey),
		WithCallerKey(c.callerKey),
		WithFunctionKey(c.functionKey),
		WithMessageKey(c.messageKey),
		WithErrorKey(c.errorKey),
		WithStacktraceKey(c.stacktraceKey),
		WithLineEnding(c.lineEnding),
		WithEncoder(c.encoder),
		WithTimeEncoder(c.timeEncoder),
		WithLevelEncoder(c.levelEncoder),
		WithDurationEncoder(c.durationEncoder),
		WithCallerEncoder(c.callerEncoder),
		WithCallerEnabled(c.enableCaller),
		WithStacktraceEnabled(c.enableStacktrace),
		WithSampler(c.sampleTick, c.sampleFirst, c.sampleThereafter, c.sampleOpts...),
		WithDevelopmentOptions(c.development),
	}
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
