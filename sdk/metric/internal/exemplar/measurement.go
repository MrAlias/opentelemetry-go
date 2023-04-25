// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exemplar // import "go.opentelemetry.io/otel/sdk/metric/internal/exemplar"

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/trace"
)

// Measurement is a measurement made by a telemetry system.
type Measurement[N int64 | float64] struct {
	Attributes  attribute.Set
	Time        time.Time
	Value       N
	SpanContext trace.SpanContext

	// Dropped are the attributes filtered out of this Measurement.
	Dropped []attribute.KeyValue

	valid bool
}

// NewMeasurement returns a new non-empty Measurement.
func NewMeasurement[N int64 | float64](ctx context.Context, ts time.Time, v N, a attribute.Set) Measurement[N] {
	return Measurement[N]{
		Attributes:  a,
		Time:        ts,
		Value:       v,
		SpanContext: trace.SpanContextFromContext(ctx),
		valid:       true,
	}
}

// Empty returns false if m represents a measurement made by a telemetry
// system, otherwise it returns true when m is its zero-value.
func (m Measurement[N]) Empty() bool { return !m.valid }

// Exemplar returns m as a [metricdata.Exemplar].
func (m Measurement[N]) Exemplar() metricdata.Exemplar[N] {
	out := metricdata.Exemplar[N]{
		FilteredAttributes: m.Dropped,
		Time:               m.Time,
		Value:              m.Value,
	}

	if m.SpanContext.HasTraceID() {
		traceID := m.SpanContext.TraceID()
		out.TraceID = traceID[:]
	}

	if m.SpanContext.HasSpanID() {
		spanID := m.SpanContext.SpanID()
		out.SpanID = spanID[:]
	}

	return out
}
