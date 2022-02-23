// SPDX-License-Identifier: BSD-3-Clause
//
// Copyright 2020 Andy Bursavich. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package zapr

import (
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

// Observer represent the ability to observe log metrics.
type Observer interface {
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

type observerEncoder struct {
	zapcore.Encoder
	observer Observer
}

func (enc *observerEncoder) Clone() zapcore.Encoder {
	return &observerEncoder{
		Encoder:  enc.Encoder.Clone(),
		observer: enc.observer,
	}
}

func (enc *observerEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	b, err := enc.Encoder.EncodeEntry(entry, fields)
	if err != nil {
		enc.observer.ObserveEncoderError(entry.LoggerName)
		return nil, err
	}
	enc.observer.ObserveEntryLogged(entry.LoggerName, entry.Level.String(), b.Len())
	return b, err
}

func loggerName(log *zap.Logger) string {
	return log.Check(zapcore.FatalLevel, "").LoggerName
}
