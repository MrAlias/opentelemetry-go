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

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type Input[N int64 | float64] func(context.Context, N, attribute.Set)

var bgCtx = context.Background()

func (f Input[N]) Async(v N, a attribute.Set) { f(bgCtx, v, a) }

type Output func(dest *metricdata.Aggregation)

type function[N int64 | float64, A aggregate[N]] interface {
	input(measurement N, attr attribute.Set)
	output(dest *[]A)
}

type aggregate[N int64 | float64] interface {
	metricdata.DataPoint[N] | metricdata.HistogramDataPoint[N]
}
