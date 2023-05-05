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

func newSum[N int64 | float64](a function[N, metricdata.DataPoint[N]], r *reservoir[N, metricdata.DataPoint[N]], mono bool, t metricdata.Temporality, f attribute.Filter) (Input[N], Output) {
	var (
		in  Input[N]
		set func(*metricdata.Sum[N])
	)
	if r == nil {
		in = func(_ context.Context, n N, attr attribute.Set) { a.input(n, attr) }
		set = func(dest *metricdata.Sum[N]) {
			dest.Temporality = t
			dest.IsMonotonic = mono
			a.output(&dest.DataPoints)
		}
	} else {
		in = func(ctx context.Context, n N, attr attribute.Set) {
			var wg sync.WaitGroup

			wg.Add(1)
			go func(t time.Time) {
				defer wg.Done()
				r.offer(ctx, t, n, attr)
			}(now())

			wg.Add(1)
			go func() {
				defer wg.Done()
				a.input(n, attr)
			}()

			wg.Wait()
		}
		set = func(dest *metricdata.Sum[N]) {
			dest.Temporality = t
			dest.IsMonotonic = mono
			a.output(&dest.DataPoints)
			r.exemplars(newIterator(dest.DataPoints))
		}
	}

	var out Output
	if f == nil {
		out = func(dest *metricdata.Aggregation) {
			// Ignore if dest is not a metricdata.Sum. The chance for memory
			// reuse of the DataPoints is missed (better luck next time).
			sData, _ := (*dest).(metricdata.Sum[N])
			set(&sData)
			*dest = sData
		}
	} else {
		fltr := filterDPtFn(f, foldSum[N])
		out = func(dest *metricdata.Aggregation) {
			sData, _ := (*dest).(metricdata.Sum[N])
			set(&sData)
			sData.DataPoints = fltr(sData.DataPoints)
			*dest = sData
		}
	}
	return in, out
}

func foldSum[N int64 | float64](a, b metricdata.DataPoint[N]) metricdata.DataPoint[N] {
	// Assumes attributes and time are the same given these are assumed sums
	// from the same collection cycle.
	if a.StartTime.After(b.StartTime) {
		a.StartTime = b.StartTime
	}
	a.Value += b.Value
	a.Exemplars = append(a.Exemplars, b.Exemplars...)
	return a
}

func newDeltaSum[N int64 | float64](monotonic bool) *deltaSum[N] {
	return &deltaSum[N]{
		values:    make(map[attribute.Set]N),
		monotonic: monotonic,
		start:     now(),
	}
}

// deltaSum summarizes a set of measurements made in a single aggregation
// cycle as their arithmetic sum.
type deltaSum[N int64 | float64] struct {
	sync.Mutex
	values map[attribute.Set]N

	monotonic bool
	start     time.Time
}

func (s *deltaSum[N]) input(value N, attr attribute.Set) {
	s.Lock()
	s.values[attr] += value
	s.Unlock()
}

func (s *deltaSum[N]) output(dest *[]metricdata.DataPoint[N]) {
	s.Lock()
	defer s.Unlock()

	t := now()
	*dest = minCapEmpty(*dest, len(s.values))
	for attr, value := range s.values {
		*dest = append(*dest, metricdata.DataPoint[N]{
			Attributes: attr,
			StartTime:  s.start,
			Time:       t,
			Value:      value,
		})
		// Unused attribute sets do not report.
		delete(s.values, attr)
	}
	// The delta collection cycle resets.
	s.start = t
}

func newCumulativeSum[N int64 | float64](monotonic bool) *cumulativeSum[N] {
	return &cumulativeSum[N]{
		values:    make(map[attribute.Set]N),
		monotonic: monotonic,
		start:     now(),
	}
}

// cumulativeSum summarizes a set of measurements made over all aggregation
// cycles as their arithmetic sum.
type cumulativeSum[N int64 | float64] struct {
	sync.Mutex
	values map[attribute.Set]N

	monotonic bool
	start     time.Time
}

func (s *cumulativeSum[N]) input(value N, attr attribute.Set) {
	s.Lock()
	s.values[attr] += value
	s.Unlock()
}

func (s *cumulativeSum[N]) output(dest *[]metricdata.DataPoint[N]) {
	s.Lock()
	defer s.Unlock()

	t := now()
	*dest = minCapEmpty(*dest, len(s.values))
	for attr, value := range s.values {
		*dest = append(*dest, metricdata.DataPoint[N]{
			Attributes: attr,
			StartTime:  s.start,
			Time:       t,
			Value:      value,
		})
		// TODO (#3006): This will use an unbounded amount of memory if there
		// are unbounded number of attribute sets being aggregated. Attribute
		// sets that become "stale" need to be forgotten so this will not
		// overload the system.
	}
}

func newPrecomputedDeltaSum[N int64 | float64](monotonic bool) *precomputedDeltaSum[N] {
	return &precomputedDeltaSum[N]{
		values:    make(map[attribute.Set]N),
		reported:  make(map[attribute.Set]N),
		monotonic: monotonic,
		start:     now(),
	}
}

// precomputedDeltaSum summarizes a set of pre-computed sums recorded over all
// aggregation cycles as the delta of these sums.
type precomputedDeltaSum[N int64 | float64] struct {
	sync.Mutex
	values   map[attribute.Set]N
	reported map[attribute.Set]N

	monotonic bool
	start     time.Time
}

func (s *precomputedDeltaSum[N]) input(value N, attr attribute.Set) {
	s.Lock()
	s.values[attr] = value
	s.Unlock()
}

func (s *precomputedDeltaSum[N]) output(dest *[]metricdata.DataPoint[N]) {
	s.Lock()
	defer s.Unlock()

	t := now()
	*dest = minCapEmpty(*dest, len(s.values))
	for attr, value := range s.values {
		delta := value - s.reported[attr]
		*dest = append(*dest, metricdata.DataPoint[N]{
			Attributes: attr,
			StartTime:  s.start,
			Time:       t,
			Value:      delta,
		})
		if delta != 0 {
			s.reported[attr] = value
		}
		// TODO (#3006): This will use an unbounded amount of memory if there
		// are unbounded number of attribute sets being aggregated. Attribute
		// sets that become "stale" need to be forgotten so this will not
		// overload the system.
	}
	// The delta collection cycle resets.
	s.start = t
}

func newPrecomputedCumulativeSum[N int64 | float64](monotonic bool) *precomputedCumulativeSum[N] {
	return &precomputedCumulativeSum[N]{newCumulativeSum[N](monotonic)}
}

// precomputedCumulativeSum directly records and reports a set of pre-computed sums.
type precomputedCumulativeSum[N int64 | float64] struct {
	*cumulativeSum[N]
}

func (s *precomputedCumulativeSum[N]) input(value N, attr attribute.Set) {
	s.Lock()
	s.values[attr] = value
	s.Unlock()
}
