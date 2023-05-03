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

package aggregate // import "go.opentelemetry.io/otel/sdk/metric/internal/aggregate"

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/internal/exemplar"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type Builder[N int64 | float64] struct {
	// TODO: doc cumulative default temporality.
	Temporality metricdata.Temporality
	Filter      attribute.Filter
	Reservoir   exemplar.Reservoir[N]
}

func (b Builder[N]) LastValue() (Input[N], Output) {
	return newLastValue[N](b.Filter)
}

func (b Builder[N]) ExplicitBucketHistogram(cfg aggregation.ExplicitBucketHistogram) (Input[N], Output) {
	return newHistogram[N](cfg, b.Temporality, b.Filter)
}

func (b Builder[N]) PrecomputedSum(monotonic bool) (Input[N], Output) {
	var a function[N, metricdata.DataPoint[N]]
	switch b.Temporality {
	case metricdata.DeltaTemporality:
		a = newPrecomputedDeltaSum[N](monotonic)
	default:
		a = newPrecomputedCumulativeSum[N](monotonic)
	}
	return newSum(a, monotonic, b.Temporality, b.Filter)
}

func (b Builder[N]) Sum(monotonic bool) (Input[N], Output) {
	var a function[N, metricdata.DataPoint[N]]
	switch b.Temporality {
	case metricdata.DeltaTemporality:
		a = newDeltaSum[N](monotonic)
	default:
		a = newCumulativeSum[N](monotonic)
	}
	return newSum(a, monotonic, b.Temporality, b.Filter)
}
