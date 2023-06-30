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
	"testing"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
)

func TestSum(t *testing.T) {
	t.Cleanup(mockTime(now))
	t.Run("Int64", testSum[int64])
	t.Run("Float64", testSum[float64])
}

func testSum[N int64 | float64](t *testing.T) {
	tester := &aggregatorTester[N]{
		GoroutineN:   defaultGoroutines,
		MeasurementN: defaultMeasurements,
		CycleN:       defaultCycles,
	}

	var agg metricdata.Aggregation = metricdata.Sum[N]{}
	t.Run("Delta", func(t *testing.T) {
		b := Builder[N]{Temporality: metricdata.DeltaTemporality}
		incr, mono := monoIncr[N](), true
		in, out := b.Sum(mono)
		eFunc := deltaExpecter[N](incr, mono)
		t.Run("Monotonic", tester.Run(in, out, incr, eFunc, &agg))

		incr, mono = nonMonoIncr[N](), false
		in, out = b.Sum(mono)
		eFunc = deltaExpecter[N](incr, mono)
		t.Run("NonMonotonic", tester.Run(in, out, incr, eFunc, &agg))
	})

	t.Run("Cumulative", func(t *testing.T) {
		b := Builder[N]{Temporality: metricdata.CumulativeTemporality}
		incr, mono := monoIncr[N](), true
		in, out := b.Sum(mono)
		eFunc := cumuExpecter[N](incr, mono)
		t.Run("Monotonic", tester.Run(in, out, incr, eFunc, &agg))

		incr, mono = nonMonoIncr[N](), false
		in, out = b.Sum(mono)
		eFunc = cumuExpecter[N](incr, mono)
		t.Run("NonMonotonic", tester.Run(in, out, incr, eFunc, &agg))
	})

	t.Run("PreComputedDelta", func(t *testing.T) {
		b := Builder[N]{Temporality: metricdata.DeltaTemporality}
		incr, mono := monoIncr[N](), true
		in, out := b.PrecomputedSum(mono)
		eFunc := preDeltaExpecter[N](incr, mono)
		t.Run("Monotonic", tester.Run(in, out, incr, eFunc, &agg))

		incr, mono = nonMonoIncr[N](), false
		in, out = b.PrecomputedSum(mono)
		eFunc = preDeltaExpecter[N](incr, mono)
		t.Run("NonMonotonic", tester.Run(in, out, incr, eFunc, &agg))
	})

	t.Run("PreComputedCumulative", func(t *testing.T) {
		b := Builder[N]{Temporality: metricdata.CumulativeTemporality}
		incr, mono := monoIncr[N](), true
		in, out := b.PrecomputedSum(mono)
		eFunc := preCumuExpecter[N](incr, mono)
		t.Run("Monotonic", tester.Run(in, out, incr, eFunc, &agg))

		incr, mono = nonMonoIncr[N](), false
		in, out = b.PrecomputedSum(mono)
		eFunc = preCumuExpecter[N](incr, mono)
		t.Run("NonMonotonic", tester.Run(in, out, incr, eFunc, &agg))
	})
}

func deltaExpecter[N int64 | float64](incr setMap[N], mono bool) expectFunc {
	sum := metricdata.Sum[N]{Temporality: metricdata.DeltaTemporality, IsMonotonic: mono}
	return func(m int) metricdata.Aggregation {
		sum.DataPoints = make([]metricdata.DataPoint[N], 0, len(incr))
		for a, v := range incr {
			sum.DataPoints = append(sum.DataPoints, point(a, v*N(m)))
		}
		return sum
	}
}

func cumuExpecter[N int64 | float64](incr setMap[N], mono bool) expectFunc {
	var cycle N
	sum := metricdata.Sum[N]{Temporality: metricdata.CumulativeTemporality, IsMonotonic: mono}
	return func(m int) metricdata.Aggregation {
		cycle++
		sum.DataPoints = make([]metricdata.DataPoint[N], 0, len(incr))
		for a, v := range incr {
			sum.DataPoints = append(sum.DataPoints, point(a, v*cycle*N(m)))
		}
		return sum
	}
}

func preDeltaExpecter[N int64 | float64](incr setMap[N], mono bool) expectFunc {
	sum := metricdata.Sum[N]{Temporality: metricdata.DeltaTemporality, IsMonotonic: mono}
	last := make(map[attribute.Set]N)
	return func(int) metricdata.Aggregation {
		sum.DataPoints = make([]metricdata.DataPoint[N], 0, len(incr))
		for a, v := range incr {
			l := last[a]
			sum.DataPoints = append(sum.DataPoints, point(a, N(v)-l))
			last[a] = N(v)
		}
		return sum
	}
}

func preCumuExpecter[N int64 | float64](incr setMap[N], mono bool) expectFunc {
	sum := metricdata.Sum[N]{Temporality: metricdata.CumulativeTemporality, IsMonotonic: mono}
	return func(int) metricdata.Aggregation {
		sum.DataPoints = make([]metricdata.DataPoint[N], 0, len(incr))
		for a, v := range incr {
			sum.DataPoints = append(sum.DataPoints, point(a, N(v)))
		}
		return sum
	}
}

// point returns a DataPoint that started and ended now.
func point[N int64 | float64](a attribute.Set, v N) metricdata.DataPoint[N] {
	return metricdata.DataPoint[N]{
		Attributes: a,
		StartTime:  now(),
		Time:       now(),
		Value:      N(v),
	}
}

func testDeltaSumReset[N int64 | float64](t *testing.T) {
	t.Cleanup(mockTime(now))

	a := newSum[N]()
	var data []metricdata.DataPoint[N]
	a.delta(&data)
	assert.Len(t, data, 0)

	a.input(context.Background(), 1, alice)
	expect := metricdata.Sum[N]{
		DataPoints: []metricdata.DataPoint[N]{point[N](alice, 1)},
	}
	a.delta(&data)
	got := metricdata.Sum[N]{DataPoints: data}
	metricdatatest.AssertAggregationsEqual(t, expect, got)

	// The attr set should be forgotten once Aggregations is called.
	expect.DataPoints = nil
	a.delta(&data)
	assert.Len(t, data, 0)

	// Aggregating another set should not affect the original (alice).
	a.input(context.Background(), 1, bob)
	expect.DataPoints = []metricdata.DataPoint[N]{point[N](bob, 1)}
	a.delta(&data)
	got = metricdata.Sum[N]{DataPoints: data}
	metricdatatest.AssertAggregationsEqual(t, expect, got)
}

func TestDeltaSumReset(t *testing.T) {
	t.Run("Int64", testDeltaSumReset[int64])
	t.Run("Float64", testDeltaSumReset[float64])
}

func TestPreComputedDeltaSum(t *testing.T) {
	agg := newPrecomputedSum[int64]()

	attrs := attribute.NewSet(attribute.String("key", "val"))
	agg.input(context.Background(), 1, attrs)
	var got []metricdata.DataPoint[int64]
	agg.delta(&got)
	opt := metricdatatest.IgnoreTimestamp()
	metricdatatest.AssertEqual(t, point[int64](attrs, 1), got[0], opt)

	// Values should persist.
	agg.delta(&got)
	metricdatatest.AssertEqual(t, point[int64](attrs, 0), got[0], opt)

	// Override set value.
	agg.input(context.Background(), 5, attrs)
	agg.input(context.Background(), 10, attrs)
	agg.delta(&got)
	metricdatatest.AssertEqual(t, point[int64](attrs, 9), got[0], opt)
}

func TestPreComputedCumulativeSum(t *testing.T) {
	agg := newPrecomputedSum[int64]()

	attrs := attribute.NewSet(attribute.String("key", "val"))
	agg.input(context.Background(), 1, attrs)
	var got []metricdata.DataPoint[int64]
	agg.cumulative(&got)
	opt := metricdatatest.IgnoreTimestamp()
	metricdatatest.AssertEqual(t, point[int64](attrs, 1), got[0], opt)

	// Cumulative values should persist.
	agg.cumulative(&got)
	metricdatatest.AssertEqual(t, point[int64](attrs, 1), got[0], opt)

	// Override set value.
	agg.input(context.Background(), 5, attrs)
	agg.input(context.Background(), 10, attrs)
	agg.cumulative(&got)
	metricdatatest.AssertEqual(t, point[int64](attrs, 10), got[0], opt)
}

func BenchmarkSum(b *testing.B) {
	b.Run("Int64", benchmarkSum[int64])
	b.Run("Float64", benchmarkSum[float64])
}

func benchmarkSum[N int64 | float64](b *testing.B) {
	// The monotonic argument is only used to annotate the Sum returned from
	// the Aggregation method. It should not have an effect on operational
	// performance, therefore, only monotonic=false is benchmarked here.
	factory := func() (Input[N], Output) {
		b := Builder[N]{Temporality: metricdata.DeltaTemporality}
		return b.Sum(false)
	}
	b.Run("Delta", benchmarkAggregator(factory))

	factory = func() (Input[N], Output) {
		b := Builder[N]{Temporality: metricdata.CumulativeTemporality}
		return b.Sum(false)
	}
	b.Run("Cumulative", benchmarkAggregator(factory))
}
