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

package metric

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument"
)

// histogram is an instrument used to recording a distribution of values.
type histogram[N int64 | float64] struct {
	instrument.Synchronous

	name string
	opts []instrument.Option
}

func newHistogram[N int64 | float64](name string, opts []instrument.Option) *histogram[N] {
	return &histogram[N]{
		name: name,
		opts: opts,
	}
}

// Record adds an additional value to the distribution.
func (c *histogram[N]) Record(ctx context.Context, incr N, attrs ...attribute.KeyValue) {
}
