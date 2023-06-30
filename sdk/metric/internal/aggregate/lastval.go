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
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func newLastValue[N int64 | float64]() *lastValue[N] {
	return &lastValue[N]{
		values: make(map[attribute.Distinct]struct {
			attr      attribute.Set
			timestamp time.Time
			value     N
		}),
	}
}

// lastValue summarizes a set of measurements as the last one made.
type lastValue[N int64 | float64] struct {
	sync.Mutex

	values map[attribute.Distinct]struct {
		attr      attribute.Set
		timestamp time.Time
		value     N
	}
}

func (s *lastValue[N]) input(ctx context.Context, value N, attr attribute.Set) {
	t := now()
	key := attr.Equivalent()

	s.Lock()
	defer s.Unlock()

	d, ok := s.values[key]
	if !ok {
		d.attr = attr
	}

	d.timestamp = t
	d.value = value

	s.values[key] = d
}

func (s *lastValue[N]) output(dest *[]metricdata.DataPoint[N]) {
	s.Lock()
	defer s.Unlock()

	n := len(s.values)
	*dest = reset(*dest, n, n)

	var i int
	for a, v := range s.values {
		(*dest)[i].Attributes = v.attr
		// The event time is the only meaningful timestamp, StartTime is
		// ignored.
		(*dest)[i].Time = v.timestamp
		(*dest)[i].Value = v.value
		// Do not report stale values.
		delete(s.values, a)
		i++
	}
}
