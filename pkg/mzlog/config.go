package mzlog

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewZapLogger returns a new zap.Logger with the given config.
func NewZapLogger(c *Config) *zap.Logger {
	var opts []zap.Option
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
	ws := c.WriteSyncer
	if ws == nil {
		ws = zapcore.Lock(os.Stderr)
	}
	return zap.New(zapcore.NewCore(c.encoder(), ws, c.Level), opts...)
}

// Config specifies the configuration of a Logger.
type Config struct {
	Level zapcore.Level

	CallerKey     string
	LevelKey      string
	MessageKey    string
	NameKey       string
	TimeKey       string
	StacktraceKey string

	Encoder      EncoderType
	TimeEncoder  TimeEncoderType
	LevelEncoder LevelEncoderType

	EnableStacktrace bool
	EnableCaller     bool

	SampleInitial    int
	SampleThereafter int

	Metrics     *Metrics
	WriteSyncer zapcore.WriteSyncer
}

// DefaultConfig returns the default Config.
func DefaultConfig() *Config {
	return &Config{
		TimeKey:          "time",
		LevelKey:         "level",
		NameKey:          "source",
		CallerKey:        "caller",
		MessageKey:       "msg",
		StacktraceKey:    "stacktrace",
		Encoder:          JSONType,
		TimeEncoder:      ISO8601Type,
		LevelEncoder:     UppercaseType,
		EnableStacktrace: true,
		EnableCaller:     true,
		SampleInitial:    100,
		SampleThereafter: 100,
		Metrics:          defaultMetrics,
	}
}

// RegisterCommonFlags registers basic fields of the Config as flags in the
// FlagSet. If fs is nil, flag.CommandLine is used.
func (c *Config) RegisterCommonFlags(fs *flag.FlagSet) *Config {
	if fs == nil {
		fs = flag.CommandLine
	}
	fs.Var(&c.Level, "log-level", "Log level.")
	fs.Var(&c.Encoder, "log-format", `Log format (e.g. "json" or "console").`)
	fs.Var(&c.TimeEncoder, "log-time-format", `Log time format (e.g. "iso8601", "millis", "nanos", or "secs").`)
	fs.Var(&c.LevelEncoder, "log-level-format", `Log level format (e.g. "upper", "lower", or "color").`)
	return c
}

// RegisterFlags registers fields of the Config as flags in the FlagSet.
// If fs is nil, flag.CommandLine is used.
func (c *Config) RegisterFlags(fs *flag.FlagSet) *Config {
	if fs == nil {
		fs = flag.CommandLine
	}
	fs.StringVar(&c.TimeKey, "log-time-key", c.TimeKey, "Log time key.")
	fs.StringVar(&c.LevelKey, "log-level-key", c.LevelKey, "Log level key.")
	fs.StringVar(&c.MessageKey, "log-message-key", c.MessageKey, "Log message key.")
	fs.StringVar(&c.CallerKey, "log-caller-key", c.CallerKey, "Log caller key.")
	fs.StringVar(&c.StacktraceKey, "log-stacktrace-key", c.StacktraceKey, "Log stacktrace key.")
	fs.BoolVar(&c.EnableStacktrace, "log-stacktrace", c.EnableStacktrace, `Log stacktrace on error or higher levels.`)
	fs.BoolVar(&c.EnableCaller, "log-caller", c.EnableCaller, `Log caller file and line.`)
	fs.IntVar(&c.SampleInitial, "log-sample-initial", c.SampleInitial, "Log every call up to this count per second.")
	fs.IntVar(&c.SampleThereafter, "log-sample-thereafter", c.SampleThereafter, "Log only one of this many calls after reaching the initial sample per second.")
	return c.RegisterCommonFlags(fs)
}

func (c *Config) encoder() zapcore.Encoder {
	cfg := zapcore.EncoderConfig{
		TimeKey:        c.TimeKey,
		LevelKey:       c.LevelKey,
		NameKey:        c.NameKey,
		CallerKey:      c.CallerKey,
		MessageKey:     c.MessageKey,
		StacktraceKey:  c.StacktraceKey,
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	switch c.LevelEncoder {
	case LowercaseType:
		cfg.EncodeLevel = zapcore.LowercaseLevelEncoder
	case ColorType:
		cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	default: // case UppercaseType:
		cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	}
	switch c.TimeEncoder {
	case MillisecondsType:
		cfg.EncodeTime = zapcore.EpochMillisTimeEncoder
	case NanosecondsType:
		cfg.EncodeTime = zapcore.EpochNanosTimeEncoder
	case SecondsType:
		cfg.EncodeTime = zapcore.EpochTimeEncoder
	default: // case ISO8601Type:
		cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	}
	var enc zapcore.Encoder
	switch c.Encoder {
	case ConsoleType:
		enc = zapcore.NewConsoleEncoder(cfg)
	default: // case JSONType:
		enc = zapcore.NewJSONEncoder(cfg)
	}
	if c.Metrics == nil {
		return enc
	}
	return &metricsEncoder{
		Encoder: enc,
		metrics: c.Metrics,
	}
}

// An EncoderType specifies which Encoder to use.
type EncoderType int

const (
	// JSONType creates a fast, low-allocation JSON encoder.
	JSONType EncoderType = iota

	// ConsoleType creates an encoder whose output is designed for human
	// consumption, rather than machine consumption.
	ConsoleType
)

// Get implements the flag.Getter interface.
func (t *EncoderType) Get() interface{} { return *t }

// Set implements the flag.Value interface.
func (t *EncoderType) Set(s string) error {
	switch strings.ToLower(s) {
	case "json":
		*t = JSONType
	case "console":
		*t = ConsoleType
	default:
		return fmt.Errorf("unknown encoder: %q", s)
	}
	return nil
}

// String implements the flag.Value interface.
func (t *EncoderType) String() string {
	switch v := *t; v {
	case JSONType:
		return "json"
	case ConsoleType:
		return "console"
	default:
		return fmt.Sprintf("Encoder(%d)", v)
	}
}

// A TimeEncoderType specifies which TimeEncoder to use.
type TimeEncoderType int

const (
	// ISO8601Type serializes a time.Time to an ISO8601-formatted string with
	// millisecond precision.
	ISO8601Type TimeEncoderType = iota

	// MillisecondsType serializes a time.Time to a floating-point number of
	// milliseconds since the Unix epoch.
	MillisecondsType

	// NanosecondsType serializes a time.Time to an integer number of nanoseconds
	// since the Unix epoch.
	NanosecondsType

	// SecondsType serializes a time.Time to a floating-point number of seconds
	// since the Unix epoch.
	SecondsType
)

// Get implements the flag.Getter interface.
func (t *TimeEncoderType) Get() interface{} { return *t }

// Set implements the flag.Value interface.
func (t *TimeEncoderType) Set(s string) error {
	switch strings.ToLower(s) {
	case "iso8601":
		*t = ISO8601Type
	case "ms", "millis":
		*t = MillisecondsType
	case "ns", "nanos":
		*t = NanosecondsType
	case "s", "secs":
		*t = SecondsType
	default:
		return fmt.Errorf("unknown time encoder: %q", s)
	}
	return nil
}

// String implements the flag.Value interface.
func (t *TimeEncoderType) String() string {
	switch v := *t; v {
	case ISO8601Type:
		return "iso8601"
	case MillisecondsType:
		return "millis"
	case NanosecondsType:
		return "nanos"
	case SecondsType:
		return "secs"
	default:
		return fmt.Sprintf("TimeEncoder(%d)", v)
	}
}

// A LevelEncoderType specifies which LevelEncoder to use.
type LevelEncoderType int

const (
	// UppercaseType serializes a Level to an all-caps string. For example,
	// InfoLevel is serialized to "INFO".
	UppercaseType LevelEncoderType = iota

	// LowercaseType serializes a Level to a lowercase string. For example,
	// InfoLevel is serialized to "info".
	LowercaseType

	// ColorType serializes a Level to an all-caps string and adds color.
	// For example, InfoLevel is serialized to "INFO" and colored blue.
	ColorType
)

// Get implements the flag.Getter interface.
func (t *LevelEncoderType) Get() interface{} { return *t }

// Set implements the flag.Value interface.
func (t *LevelEncoderType) Set(s string) error {
	switch strings.ToLower(s) {
	case "upper", "uppercase":
		*t = UppercaseType
	case "lower", "lowercase":
		*t = LowercaseType
	case "color":
		*t = ColorType
	default:
		return fmt.Errorf("unknown level encoder: %q", s)
	}
	return nil
}

// String implements the flag.Value interface.
func (t *LevelEncoderType) String() string {
	switch v := *t; v {
	case UppercaseType:
		return "upper"
	case LowercaseType:
		return "lower"
	case ColorType:
		return "color"
	default:
		return fmt.Sprintf("LevelEncoder(%d)", v)
	}
}
