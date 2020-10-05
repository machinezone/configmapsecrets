// Copyright 2020 Andy Bursavich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package zapr

import (
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

// Metrics represent the ability to observe log metrics.
type Metrics interface {
	// Init initializes metrics for the named logger when it's created.
	// Logger names are not required to be unique and it may be called
	// with a duplicate name at any time.
	Init(logger string)

	// ObserveEntryLogged observes logged entry metrics for the named logger, at
	// the given level, and with the given bytes.
	ObserveEntryLogged(logger string, level string, bytes int)

	// ObserveEncoderError observes an error encoding an entry for the named logger.
	ObserveEncoderError(logger string)
}

// PrometheusMetrics represent the ability to observe metrics for Prometheus.
type PrometheusMetrics interface {
	prometheus.Collector
	Metrics
}

// NewPrometheusMetrics returns new PrometheusMetrics.
func NewPrometheusMetrics() PrometheusMetrics {
	return &promMetrics{
		lines: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "log_lines_total",
				Help: "Total number of log lines.",
			},
			[]string{"name", "level"},
		),
		bytes: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "log_bytes_total",
				Help: "Total bytes of encoded log lines.",
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

type promMetrics struct {
	lines  *prometheus.CounterVec
	bytes  *prometheus.CounterVec
	errors *prometheus.CounterVec
}

func (m *promMetrics) Describe(ch chan<- *prometheus.Desc) {
	m.lines.Describe(ch)
	m.bytes.Describe(ch)
	m.errors.Describe(ch)
}

func (m *promMetrics) Collect(ch chan<- prometheus.Metric) {
	m.lines.Collect(ch)
	m.bytes.Collect(ch)
	m.errors.Collect(ch)
}

func (m *promMetrics) Init(logger string) {
	for _, lvl := range []zapcore.Level{zap.InfoLevel, zap.ErrorLevel} {
		m.lines.WithLabelValues(logger, lvl.String())
		m.bytes.WithLabelValues(logger, lvl.String())
	}
	m.errors.WithLabelValues(logger)
}

func (m *promMetrics) ObserveEntryLogged(logger string, level string, bytes int) {
	m.bytes.WithLabelValues(logger, level).Add(float64(bytes))
	m.lines.WithLabelValues(logger, level).Inc()
}

func (m *promMetrics) ObserveEncoderError(logger string) {
	m.errors.WithLabelValues(logger).Inc()
}

type metricsEncoder struct {
	zapcore.Encoder
	metrics Metrics
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
		enc.metrics.ObserveEncoderError(entry.LoggerName)
		return nil, err
	}
	enc.metrics.ObserveEntryLogged(entry.LoggerName, entry.Level.String(), b.Len())
	return b, err
}

func loggerName(log *zap.Logger) string {
	return log.Check(zapcore.FatalLevel, "").LoggerName
}
