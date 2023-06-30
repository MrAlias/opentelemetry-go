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

func newSum[N int64 | float64]() *sum[N] {
	return &sum[N]{
		values: make(map[attribute.Distinct]struct {
			attr attribute.Set
			n    N
		}),
		start: now(),
	}
}

// sum summarizes a set of measurements made in a single aggregation
// cycle as their arithmetic sum.
type sum[N int64 | float64] struct {
	sync.Mutex

	values map[attribute.Distinct]struct {
		attr attribute.Set
		n    N
	}

	start time.Time
}

func (s *sum[N]) input(ctx context.Context, value N, attr attribute.Set) {
	key := attr.Equivalent()

	s.Lock()
	defer s.Unlock()

	v, ok := s.values[key]
	if !ok {
		v.attr = attr
	}

	v.n += value

	s.values[key] = v
}

func (s *sum[N]) delta(dest *[]metricdata.DataPoint[N]) {
	t := now()

	s.Lock()
	defer s.Unlock()

	n := len(s.values)
	*dest = reset(*dest, n, n)

	var i int
	for key, val := range s.values {
		(*dest)[i].Attributes = val.attr
		(*dest)[i].StartTime = s.start
		(*dest)[i].Time = t
		(*dest)[i].Value = val.n
		// Do not report stale values.
		delete(s.values, key)
		i++
	}
	// The delta collection cycle resets.
	s.start = t
}

func (s *sum[N]) cumulative(dest *[]metricdata.DataPoint[N]) {
	t := now()

	s.Lock()
	defer s.Unlock()

	n := len(s.values)
	*dest = reset(*dest, n, n)

	var i int
	for _, val := range s.values {
		(*dest)[i].Attributes = val.attr
		(*dest)[i].StartTime = s.start
		(*dest)[i].Time = t
		(*dest)[i].Value = val.n
		// TODO (#3006): This will use an unbounded amount of memory if there
		// are unbounded number of attribute sets being aggregated. Attribute
		// sets that become "stale" need to be forgotten so this will not
		// overload the system.
		i++
	}
}

func newPrecomputedSum[N int64 | float64]() *precomputedSum[N] {
	return &precomputedSum[N]{
		values: make(map[attribute.Distinct]struct {
			attr attribute.Set
			n    N
		}),
		start: now(),
	}
}

// precomputedSum summarizes a set of pre-computed sums recorded over all
// aggregation cycles as the delta of these sums.
type precomputedSum[N int64 | float64] struct {
	sync.Mutex
	values map[attribute.Distinct]struct {
		attr attribute.Set
		n    N
	}
	reported map[attribute.Distinct]N

	start time.Time
}

func (s *precomputedSum[N]) input(ctx context.Context, value N, attr attribute.Set) {
	key := attr.Equivalent()

	s.Lock()
	defer s.Unlock()

	v, ok := s.values[key]
	if !ok {
		v.attr = attr
	}

	v.n = value

	s.values[key] = v
}

func (s *precomputedSum[N]) delta(dest *[]metricdata.DataPoint[N]) {
	t := now()

	s.Lock()
	defer s.Unlock()

	if s.reported == nil {
		// Lazy allocated s.reported only if collecting delta values.
		s.reported = make(map[attribute.Distinct]N)
	}

	n := len(s.values)
	*dest = reset(*dest, n, n)

	var i int
	for key, val := range s.values {
		delta := val.n - s.reported[key]

		(*dest)[i].Attributes = val.attr
		(*dest)[i].StartTime = s.start
		(*dest)[i].Time = t
		(*dest)[i].Value = delta

		if delta != 0 {
			s.reported[key] = val.n
		}
		s.values[key] = val
		// TODO (#3006): This will use an unbounded amount of memory if there
		// are unbounded number of attribute sets being aggregated. Attribute
		// sets that become "stale" need to be forgotten so this will not
		// overload the system.
		i++
	}
	// The delta collection cycle resets.
	s.start = t
}

func (s *precomputedSum[N]) cumulative(dest *[]metricdata.DataPoint[N]) {
	t := now()

	s.Lock()
	defer s.Unlock()

	n := len(s.values)
	*dest = reset(*dest, n, n)

	var i int
	for key, val := range s.values {
		(*dest)[i].Attributes = val.attr
		(*dest)[i].StartTime = s.start
		(*dest)[i].Time = t
		(*dest)[i].Value = val.n
		s.values[key] = val
		// TODO (#3006): This will use an unbounded amount of memory if there
		// are unbounded number of attribute sets being aggregated. Attribute
		// sets that become "stale" need to be forgotten so this will not
		// overload the system.
		i++
	}
}
