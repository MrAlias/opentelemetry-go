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

package aggregator // import "go.opentelemetry.io/otel/sdk/metric/internal/aggregator"

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/internal/exemplar"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type exemplarAgg[N int64 | float64] struct {
	filter    exemplar.Filter[N]
	reservoir exemplar.Reservoir[N]
	wrapped   filteringAggregator[N]
}

func newExemplarAgg[N int64 | float64](f exemplar.Filter[N], r exemplar.Reservoir[N], a filteringAggregator[N]) *exemplarAgg[N] {
	return &exemplarAgg[N]{
		filter:    f,
		reservoir: r,
		wrapped:   a,
	}
}

type filteringAggregator[N int64 | float64] interface {
	Aggregator[N]

	aggregate(v N, attr attribute.Set) []attribute.KeyValue
}

func (a *exemplarAgg[N]) Aggregate(ctx context.Context, v N, attr attribute.Set) {
	var fltrA []attribute.KeyValue
	if a.filter(ctx, v, attr) {
		defer func(t time.Time) { a.reservoir.Offer(ctx, t, v, fltrA) }(now())
	}
	fltrA = a.wrapped.aggregate(v, attr)
}

// Aggregation returns an Aggregation, for all the aggregated
// measurements made and ends an aggregation cycle.
func (a *exemplarAgg[N]) Aggregation() metricdata.Aggregation {
	return a.wrapped.Aggregation()
}
