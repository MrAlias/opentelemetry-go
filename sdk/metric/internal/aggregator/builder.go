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

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/internal/exemplar"
)

type AggregatorBuilder[N int64 | float64] struct {
	aFltr attribute.Filter
	eFltr exemplar.Filter[N]
	eResF func() exemplar.Reservoir[N]
}

func NewAggregatorBuilder[N int64 | float64]() AggregatorBuilder[N] {
	return AggregatorBuilder[N]{ /*TODO: default eResF*/ }
}

func (b AggregatorBuilder[N]) WithAttributeFilter(f attribute.Filter) AggregatorBuilder[N] {
	b.aFltr = f
	return b
}

func (b AggregatorBuilder[N]) WithExemplarFilter(f exemplar.Filter[N]) AggregatorBuilder[N] {
	b.eFltr = f
	return b
}

func (b AggregatorBuilder[N]) WithExemplarReservoirFunc(f func() exemplar.Reservoir[N]) AggregatorBuilder[N] {
	b.eResF = f
	return b
}

func (b AggregatorBuilder[N]) Aggregator(a aggregation.Aggregation) Aggregator[N] {
	return b.aggregator(a)
}

func (b AggregatorBuilder[N]) ContextAggregator(a aggregation.Aggregation) ContextAggregator[N] {
	agg := b.aggregator(a)
	if b.eFltr != nil {
		fAgg, ok := agg.(filteringAggregator[N])
		if !ok {
			fAgg = fltrAgg[N]{agg}
		}
		return newExemplarAgg(b.eFltr, b.eResF(), fAgg)
	}
	return ctxAgg[N]{agg}
}

type fltrAgg[N int64 | float64] struct {
	Aggregator[N]
}

func (a fltrAgg[N]) aggregate(v N, attr attribute.Set) []attribute.KeyValue {
	a.Aggregate(v, attr)
	return nil
}

type ctxAgg[N int64 | float64] struct {
	Aggregator[N]
}

func (a ctxAgg[N]) Aggregate(_ context.Context, v N, attrs attribute.Set) {
	a.Aggregator.Aggregate(v, attrs)
}

func (b AggregatorBuilder[N]) aggregator(a aggregation.Aggregation) Aggregator[N] {
	var agg Aggregator[N]
	switch a {
	case aggregation.LastValue{}:
		agg = NewLastValue[N]()
	default:
		// FIXME: add the others
		panic("unknown aggregation")
	}
	if b.aFltr != nil {
		agg = newFilter(agg, b.aFltr)
	}
	return agg
}
