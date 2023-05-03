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
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// now is used to return the current local time while allowing tests to
// override the default time.Now function.
var now = time.Now

type Input[N int64 | float64] func(context.Context, N, attribute.Set)

var bgCtx = context.Background()

func (f Input[N]) Async(v N, a attribute.Set) { f(bgCtx, v, a) }

type Output func(dest *metricdata.Aggregation)

type function[N int64 | float64, D any] interface {
	input(measurement N, attr attribute.Set)
	output(dest *[]D)
}

type aggregator[N int64 | float64, D any] interface {
	aggregate(context.Context, N, attribute.Set)
	aggergation(dest *D)
}

func minCapEmpty[T any](v []T, n int) []T {
	if cap(v) < n {
		return make([]T, 0, n)
	}
	return v[:0]
}

func minCap[T any](v []T, n int) []T {
	if cap(v) < n {
		return make([]T, n)
	}
	return v[:n]
}
