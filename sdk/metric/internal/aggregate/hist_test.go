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
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
)

var (
	bounds   = []float64{1, 5}
	histConf = aggregation.ExplicitBucketHistogram{
		Boundaries: bounds,
		NoMinMax:   false,
	}
)

func TestHistogram(t *testing.T) {
	t.Cleanup(mockTime(now))
	t.Run("Int64", testHistogram[int64])
	t.Run("Float64", testHistogram[float64])
}

func testHistogram[N int64 | float64](t *testing.T) {
	tester := &aggregatorTester[N]{
		GoroutineN:   defaultGoroutines,
		MeasurementN: defaultMeasurements,
		CycleN:       defaultCycles,
	}

	incr := monoIncr[N]()
	eFunc := deltaHistExpecter[N](incr)
	var agg metricdata.Aggregation = metricdata.Histogram[N]{}
	b := Builder[N]{Temporality: metricdata.DeltaTemporality}
	in, out := b.ExplicitBucketHistogram(histConf)
	t.Run("Delta", tester.Run(in, out, incr, eFunc, &agg))

	b.Temporality = metricdata.CumulativeTemporality
	in, out = b.ExplicitBucketHistogram(histConf)
	eFunc = cumuHistExpecter[N](incr)
	t.Run("Cumulative", tester.Run(in, out, incr, eFunc, &agg))
}

func deltaHistExpecter[N int64 | float64](incr setMap[N]) expectFunc {
	h := metricdata.Histogram[N]{Temporality: metricdata.DeltaTemporality}
	return func(m int) metricdata.Aggregation {
		h.DataPoints = make([]metricdata.HistogramDataPoint[N], 0, len(incr))
		for a, v := range incr {
			h.DataPoints = append(h.DataPoints, hPoint[N](a, v, uint64(m)))
		}
		return h
	}
}

func cumuHistExpecter[N int64 | float64](incr setMap[N]) expectFunc {
	var cycle int
	h := metricdata.Histogram[N]{Temporality: metricdata.CumulativeTemporality}
	return func(m int) metricdata.Aggregation {
		cycle++
		h.DataPoints = make([]metricdata.HistogramDataPoint[N], 0, len(incr))
		for a, v := range incr {
			h.DataPoints = append(h.DataPoints, hPoint[N](a, v, uint64(cycle*m)))
		}
		return h
	}
}

// hPoint returns an HistogramDataPoint that started and ended now with multi
// number of measurements values v. It includes a min and max (set to v).
func hPoint[N int64 | float64](a attribute.Set, v N, multi uint64) metricdata.HistogramDataPoint[N] {
	idx := sort.SearchFloat64s(bounds, float64(v))
	counts := make([]uint64, len(bounds)+1)
	counts[idx] += multi
	return metricdata.HistogramDataPoint[N]{
		Attributes:   a,
		StartTime:    now(),
		Time:         now(),
		Count:        multi,
		Bounds:       bounds,
		BucketCounts: counts,
		Min:          metricdata.NewExtrema(v),
		Max:          metricdata.NewExtrema(v),
		Sum:          v * N(multi),
	}
}

func TestBucketsBin(t *testing.T) {
	t.Run("Int64", testBucketsBin[int64]())
	t.Run("Float64", testBucketsBin[float64]())
}

func testBucketsBin[N int64 | float64]() func(t *testing.T) {
	return func(t *testing.T) {
		b := newBuckets[N](3)
		assertB := func(counts []uint64, count uint64, sum, min, max N) {
			assert.Equal(t, counts, b.counts)
			assert.Equal(t, count, b.count)
			assert.Equal(t, sum, b.sum)
			assert.Equal(t, min, b.min)
			assert.Equal(t, max, b.max)
		}

		assertB([]uint64{0, 0, 0}, 0, 0, 0, 0)
		b.bin(1, 2)
		assertB([]uint64{0, 1, 0}, 1, 2, 0, 2)
		b.bin(0, -1)
		assertB([]uint64{1, 1, 0}, 2, 1, -1, 2)
	}
}

func testHistImmutableBounds[N int64 | float64](out func(*hist[N]) []metricdata.HistogramDataPoint[N]) func(t *testing.T) {
	b := []float64{0, 1, 2}
	cpB := make([]float64, len(b))
	copy(cpB, b)

	h := newHistogram[N](aggregation.ExplicitBucketHistogram{Boundaries: b})
	return func(t *testing.T) {
		require.Equal(t, cpB, h.bounds)

		b[0] = 10
		assert.Equal(t, cpB, h.bounds, "modifying the bounds argument should not change the bounds")

		h.input(context.Background(), 5, alice)
		hdp := out(h)[0]
		require.Len(t, hdp.Bounds, len(b))
		hdp.Bounds[1] = 10
		assert.Equal(t, cpB, h.bounds, "modifying the Aggregation bounds should not change the bounds")
	}
}

func TestHistogramImmutableBounds(t *testing.T) {
	t.Run("Delta/Int64", testHistImmutableBounds[int64](
		func(h *hist[int64]) []metricdata.HistogramDataPoint[int64] {
			var out []metricdata.HistogramDataPoint[int64]
			h.delta(&out)
			return out
		},
	))
	t.Run("Delta/Float64", testHistImmutableBounds[float64](
		func(h *hist[float64]) []metricdata.HistogramDataPoint[float64] {
			var out []metricdata.HistogramDataPoint[float64]
			h.delta(&out)
			return out
		},
	))

	t.Run("Cumulative/Int64", testHistImmutableBounds(
		func(h *hist[int64]) []metricdata.HistogramDataPoint[int64] {
			var out []metricdata.HistogramDataPoint[int64]
			h.cumulative(&out)
			return out
		},
	))
	t.Run("Cumulative/Float64", testHistImmutableBounds(
		func(h *hist[float64]) []metricdata.HistogramDataPoint[float64] {
			var out []metricdata.HistogramDataPoint[float64]
			h.cumulative(&out)
			return out
		},
	))
}

func TestCumulativeHistogramImutableCounts(t *testing.T) {
	a := newHistogram[int64](histConf)
	a.input(context.Background(), 5, alice)

	var out []metricdata.HistogramDataPoint[int64]
	a.cumulative(&out)
	hdp := out[0]
	require.Equal(t, hdp.BucketCounts, a.values[alice.Equivalent()].counts)

	cpCounts := make([]uint64, len(hdp.BucketCounts))
	copy(cpCounts, hdp.BucketCounts)
	hdp.BucketCounts[0] = 10
	assert.Equal(t, cpCounts, a.values[alice.Equivalent()].counts, "modifying the Aggregator bucket counts should not change the Aggregator")
}

func TestDeltaHistogramReset(t *testing.T) {
	t.Cleanup(mockTime(now))

	a := newHistogram[int64](histConf)
	var out []metricdata.HistogramDataPoint[int64]
	a.delta(&out)
	assert.Len(t, out, 0)

	a.input(context.Background(), 1, alice)
	a.delta(&out)
	metricdatatest.AssertEqual(t, hPoint[int64](alice, 1, 1), out[0])

	// The attr set should be forgotten once Aggregations is called.
	a.delta(&out)
	assert.Len(t, out, 0)

	// Aggregating another set should not affect the original (alice).
	a.input(context.Background(), 1, bob)
	a.delta(&out)
	metricdatatest.AssertEqual(t, hPoint[int64](bob, 1, 1), out[0])
}

func BenchmarkHistogram(b *testing.B) {
	b.Run("Int64", benchmarkHistogram[int64])
	b.Run("Float64", benchmarkHistogram[float64])
}

func benchmarkHistogram[N int64 | float64](b *testing.B) {
	build := Builder[N]{Temporality: metricdata.DeltaTemporality}
	factory := func() (Input[N], Output) { return build.ExplicitBucketHistogram(histConf) }
	b.Run("Delta", benchmarkAggregator(factory))
	build.Temporality = metricdata.CumulativeTemporality
	b.Run("Cumulative", benchmarkAggregator(factory))
}
