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
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
)

const (
	defaultGoroutines   = 5
	defaultMeasurements = 30
	defaultCycles       = 3
)

var (
	alice = attribute.NewSet(attribute.String("user", "alice"), attribute.Bool("admin", true))
	bob   = attribute.NewSet(attribute.String("user", "bob"), attribute.Bool("admin", false))
	carol = attribute.NewSet(attribute.String("user", "carol"), attribute.Bool("admin", false))

	// Sat Jan 01 2000 00:00:00 GMT+0000.
	staticTime    = time.Unix(946684800, 0)
	staticNowFunc = func() time.Time { return staticTime }
	// Pass to t.Cleanup to override the now function with staticNowFunc and
	// revert once the test completes. E.g. t.Cleanup(mockTime(now)).
	mockTime = func(orig func() time.Time) (cleanup func()) {
		now = staticNowFunc
		return func() { now = orig }
	}
)

func monoIncr[N int64 | float64]() setMap[N] {
	return setMap[N]{alice: 1, bob: 10, carol: 2}
}

func nonMonoIncr[N int64 | float64]() setMap[N] {
	return setMap[N]{alice: 1, bob: -1, carol: 2}
}

// setMap maps attribute sets to a number.
type setMap[N int64 | float64] map[attribute.Set]N

// expectFunc is a function that returns an Aggregation of expected values for
// a cycle that contains m measurements (total across all goroutines). Each
// call advances the cycle.
type expectFunc func(m int) metricdata.Aggregation

// aggregatorTester runs an acceptance test on an Aggregator. It will ask an
// Aggregator to aggregate a set of values as if they were real measurements
// made MeasurementN number of times. This will be done in GoroutineN number
// of different goroutines. After the Aggregator has been asked to aggregate
// all these measurements, it is validated using a passed expecterFunc. This
// set of operation is a single cycle, and the the aggregatorTester will run
// CycleN number of cycles.
type aggregatorTester[N int64 | float64] struct {
	// GoroutineN is the number of goroutines aggregatorTester will use to run
	// the test with.
	GoroutineN int
	// MeasurementN is the number of measurements that are made each cycle a
	// goroutine runs the test.
	MeasurementN int
	// CycleN is the number of times a goroutine will make a set of
	// measurements.
	CycleN int
}

func (at *aggregatorTester[N]) Run(in Input[N], out Output, incr setMap[N], eFunc expectFunc, agg *metricdata.Aggregation) func(*testing.T) {
	m := at.MeasurementN * at.GoroutineN
	return func(t *testing.T) {
		for i := 0; i < at.CycleN; i++ {
			var wg sync.WaitGroup
			wg.Add(at.GoroutineN)
			for j := 0; j < at.GoroutineN; j++ {
				go func() {
					defer wg.Done()
					for k := 0; k < at.MeasurementN; k++ {
						for attrs, n := range incr {
							in(context.Background(), N(n), attrs)
						}
					}
				}()
			}
			wg.Wait()

			out(agg)
			metricdatatest.AssertAggregationsEqual(t, eFunc(m), *agg)
		}
	}
}

func TestNoInputReturnsZeroOutput(t *testing.T) {
	t.Run("Int64", testNoInputReturnsZeroOutput[int64]())
	t.Run("Float64", testNoInputReturnsZeroOutput[float64]())
}

func testNoInputReturnsZeroOutput[N int64 | float64]() func(t *testing.T) {
	run := func(_ Input[N], out Output) func(t *testing.T) {
		return func(t *testing.T) {
			t.Helper()
			var data metricdata.Aggregation
			assert.Equal(t, 0, out(&data), "wrong amount of data output")
		}
	}

	return func(t *testing.T) {
		b := Builder[N]{Temporality: metricdata.DeltaTemporality}
		t.Run("Delta/LastValue", run(b.LastValue()))

		t.Run("Delta/Monotonic/Sum", run(b.Sum(true)))
		t.Run("Delta/NonMonotonic/Sum", run(b.Sum(false)))
		b.Temporality = metricdata.CumulativeTemporality
		t.Run("Cumulative/Monotonic/Sum", run(b.Sum(true)))
		t.Run("Cumulative/NonMonotonic/Sum", run(b.Sum(false)))

		t.Run("Delta/Monotonic/PrecomputedSum", run(b.PrecomputedSum(true)))
		t.Run("Delta/NonMonotonic/PrecomputedSum", run(b.PrecomputedSum(false)))
		b.Temporality = metricdata.CumulativeTemporality
		t.Run("Cumulative/Monotonic/PrecomputedSum", run(b.PrecomputedSum(true)))
		t.Run("Cumulative/NonMonotonic/PrecomputedSum", run(b.PrecomputedSum(false)))

		b.Temporality = metricdata.DeltaTemporality
		t.Run("Delta/Histogram", run(b.ExplicitBucketHistogram(histConf)))
		b.Temporality = metricdata.CumulativeTemporality
		t.Run("Delta/Histogram", run(b.ExplicitBucketHistogram(histConf)))
	}
}

var bmarkResults metricdata.Aggregation

func benchmarkAggregatorN[N int64 | float64](b *testing.B, factory func() (Input[N], Output), count int) {
	attrs := make([]attribute.Set, count)
	for i := range attrs {
		attrs[i] = attribute.NewSet(attribute.Int("value", i))
	}

	b.Run("Input", func(b *testing.B) {
		in, out := factory()
		ctx := context.Background()
		b.ReportAllocs()
		b.ResetTimer()

		for n := 0; n < b.N; n++ {
			for _, attr := range attrs {
				in(ctx, 1, attr)
			}
		}
		out(&bmarkResults)
	})

	b.Run("Output", func(b *testing.B) {
		outs := make([]Output, b.N)
		for n := range outs {
			ctx := context.Background()
			in, out := factory()
			for _, attr := range attrs {
				in(ctx, 1, attr)
			}
			outs[n] = out
		}

		b.ReportAllocs()
		b.ResetTimer()

		for n := 0; n < b.N; n++ {
			outs[n](&bmarkResults)
		}
	})
}

func benchmarkAggregator[N int64 | float64](factory func() (Input[N], Output)) func(*testing.B) {
	counts := []int{1, 10, 100}
	return func(b *testing.B) {
		for _, n := range counts {
			b.Run(strconv.Itoa(n), func(b *testing.B) {
				benchmarkAggregatorN(b, factory, n)
			})
		}
	}
}
