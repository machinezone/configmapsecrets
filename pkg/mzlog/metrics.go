package mzlog

import (
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

var defaultMetrics = NewMetrics()

// Metrics are a prometheus.Collector for log metrics.
type Metrics struct {
	entries *prometheus.CounterVec
	bytes   *prometheus.CounterVec
	errors  *prometheus.CounterVec
}

// NewMetrics returns new Metrics.
func NewMetrics() *Metrics {
	return &Metrics{
		entries: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "log_entries_total",
				Help: "Total number of log entries.",
			},
			[]string{"name", "level"},
		),
		bytes: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "log_bytes_total",
				Help: "Total bytes of encoded log entries.",
			},
			[]string{"name", "level"},
		),
		errors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "log_encoder_errors_total",
				Help: "Total number of log entry encoding failures.",
			},
			[]string{"name"},
		),
	}
}

// Describe implements the prometheus.Collector interface.
func (m *Metrics) Describe(ch chan<- *prometheus.Desc) {
	m.entries.Describe(ch)
	m.bytes.Describe(ch)
}

// Collect implements the prometheus.Collector interface.
func (m *Metrics) Collect(ch chan<- prometheus.Metric) {
	m.entries.Collect(ch)
	m.bytes.Collect(ch)
}

type metricsEncoder struct {
	zapcore.Encoder
	metrics *Metrics
}

func (enc *metricsEncoder) Clone() zapcore.Encoder {
	return &metricsEncoder{
		Encoder: enc.Encoder.Clone(),
		metrics: enc.metrics,
	}
}

func (enc *metricsEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	b, err := enc.Encoder.EncodeEntry(entry, fields)
	if err != nil {
		enc.metrics.errors.WithLabelValues(entry.LoggerName).Inc()
		origErr := err
		// NOTE: We intentionally leave the existing caller.
		entry.Level = zapcore.ErrorLevel
		entry.Message = "failed to encode log entry"
		fields = []zapcore.Field{zap.Error(err)}
		if b, err = enc.Encoder.EncodeEntry(entry, fields); err != nil {
			return nil, origErr
		}
	}
	lvl := entry.Level.String()
	enc.metrics.entries.WithLabelValues(entry.LoggerName, lvl).Inc()
	enc.metrics.bytes.WithLabelValues(entry.LoggerName, lvl).Add(float64(b.Len()))
	return b, err
}
