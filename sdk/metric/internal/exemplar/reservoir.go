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
	"math/rand"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/trace"
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

type Reservoir[N int64 | float64] interface {
	Offer(context.Context, time.Time, N, []attribute.KeyValue)
	Collect(*[]metricdata.Exemplar[N])
}

func FixedSize[N int64 | float64](n int) Reservoir[N] {
	return &fixedSize[N]{storage: make([]measurement[N], 0, n)}
}

type fixedSize[N int64 | float64] struct {
	// n is the count of measurements seen.
	n int64

	// storage are the measurements sampled.
	//
	// This does not use []metricdata.Exemplar because it potentially would
	// require an allocation for trace and span IDs in the hot path of Offer.
	storage []measurement[N]
}

func (r *fixedSize[N]) Offer(ctx context.Context, ts time.Time, v N, attr []attribute.KeyValue) {
	// TODO: fix overflow error.
	r.n++
	if len(r.storage) < cap(r.storage) {
		r.storage = append(r.storage, newMeasurement(ctx, ts, v, attr))
		return
	}

	j := rng.Int63n(r.n)
	if j < int64(cap(r.storage)) {
		r.storage[j] = newMeasurement(ctx, v, attr)
	}
}

func (r *fixedSize[N]) Collect(out *[]metricdata.Exemplar[N]) {
	*out = minCap(*out, len(r.storage))
	for i, m := range r.storage {
		m.Copy(&(*out)[i])
	}

	r.storage = r.storage[:0]
	r.n = 0
}

type measurement[N int64 | float64] struct {
	Time         time.Time
	FilteredAttr []attribute.KeyValue
	Value        N
	SpanContext  trace.SpanContext
}

func newMeasurement[N int64 | float64](ctx context.Context, ts time.Time, v N, attr []attribute.KeyValue) measurement[N] {
	return measurement[N]{
		Time:         ts,
		FilteredAttr: attr,
		Value:        v,
		SpanContext:  trace.SpanContextFromContext(ctx),
	}
}

func (m measurement[N]) Copy(dest *metricdata.Exemplar[N]) {
	dest.FilteredAttributes = m.FilteredAttr
	dest.Time = m.Time
	dest.Value = m.Value

	if m.SpanContext.HasTraceID() {
		traceID := m.SpanContext.TraceID()
		dest.TraceID = minCap(dest.TraceID, len(traceID))
		copy(dest.TraceID[:len(traceID)], traceID[:])
	}

	if m.SpanContext.HasSpanID() {
		spanID := m.SpanContext.SpanID()
		dest.SpanID = minCap(dest.SpanID, len(spanID))
		copy(dest.SpanID[:len(spanID)], spanID[:])
	}
}

func minCap[T any](slice []T, n int) []T {
	if cap(slice) < n {
		return make([]T, n)
	}
	return slice[:n]
}
