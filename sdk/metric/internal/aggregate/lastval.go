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

func newLastValue[N int64 | float64](r *reservoir[N, metricdata.DataPoint[N]], f attribute.Filter) (Input[N], Output) {
	lv := &lastValue[N]{values: make(map[attribute.Set]datapoint[N])}

	var (
		in  Input[N]
		set func(*metricdata.Gauge[N])
	)
	if r == nil {
		in = func(_ context.Context, n N, a attribute.Set) {
			lv.input(n, a)
		}
		set = func(dest *metricdata.Gauge[N]) { lv.output(&dest.DataPoints) }
	} else {
		in = func(ctx context.Context, n N, a attribute.Set) {
			var wg sync.WaitGroup

			wg.Add(1)
			go func(t time.Time) {
				defer wg.Done()
				r.offer(ctx, t, n, a)
			}(now())

			wg.Add(1)
			go func() {
				defer wg.Done()
				lv.input(n, a)
			}()

			wg.Wait()
		}
		set = func(dest *metricdata.Gauge[N]) {
			lv.output(&dest.DataPoints)
			r.exemplars(newIterator(dest.DataPoints))
		}
	}

	var out Output
	if f == nil {
		out = func(dest *metricdata.Aggregation) {
			// Ignore if dest is not a metricdata.Gauge. The chance for memory
			// reuse of the DataPoints is missed (better luck next time).
			gData, _ := (*dest).(metricdata.Gauge[N])
			set(&gData)
			*dest = gData
		}
	} else {
		fltr := filterDPtFn(f, foldGauge[N])
		out = func(dest *metricdata.Aggregation) {
			gData, _ := (*dest).(metricdata.Gauge[N])
			set(&gData)
			gData.DataPoints = fltr(gData.DataPoints)
			*dest = gData
		}
	}
	return in, out
}

func foldGauge[N int64 | float64](a, b metricdata.DataPoint[N]) metricdata.DataPoint[N] {
	// Assumes attributes are the same given these are assumed gauges from the
	// same collection cycle.
	if b.Time.After(a.Time) {
		a.Time = b.Time
		a.Value = b.Value
	}
	a.Exemplars = append(a.Exemplars, b.Exemplars...)
	return a
}

// datapoint is timestamped measurement data.
type datapoint[N int64 | float64] struct {
	timestamp time.Time
	value     N
}

// lastValue summarizes a set of measurements as the last one made.
type lastValue[N int64 | float64] struct {
	sync.Mutex

	values map[attribute.Set]datapoint[N]
}

func (s *lastValue[N]) input(value N, attr attribute.Set) {
	d := datapoint[N]{timestamp: now(), value: value}
	s.Lock()
	s.values[attr] = d
	s.Unlock()
}

func (s *lastValue[N]) output(dest *[]metricdata.DataPoint[N]) {
	s.Lock()
	defer s.Unlock()

	*dest = minCapEmpty(*dest, len(s.values))
	for a, v := range s.values {
		*dest = append(*dest, metricdata.DataPoint[N]{
			Attributes: a,
			// The event time is the only meaningful timestamp, StartTime is
			// ignored.
			Time:  v.timestamp,
			Value: v.value,
		})
		// Do not report stale values.
		delete(s.values, a)
	}
}
