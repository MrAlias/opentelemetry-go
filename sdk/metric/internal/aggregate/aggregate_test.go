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
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/internal/exemplar"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
)

var (
	keyUser    = "user"
	userAlice  = attribute.String(keyUser, "Alice")
	userBob    = attribute.String(keyUser, "Bob")
	adminTrue  = attribute.Bool("admin", true)
	adminFalse = attribute.Bool("admin", false)

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

func TestDeltaSumAggregatorExemplars(t *testing.T) {
	t.Cleanup(mockTime(now))

	alice := attribute.NewSet(userAlice, adminTrue)
	bob := attribute.NewSet(userBob, adminFalse)

	f := func(_ context.Context, v int64, attr attribute.Set) bool {
		return attr == alice && v == 2
	}
	b := Builder[int64]{
		Temporality: metricdata.DeltaTemporality,
		Filter: func(kv attribute.KeyValue) bool {
			return kv.Key == attribute.Key(keyUser)
		},
		Reservoir: exemplar.Filtered(exemplar.FixedSize[int64](10), f),
	}
	in, out := b.Sum(false)

	ctx := context.Background()
	in(ctx, 1, alice)
	in(ctx, -1, bob)
	in(ctx, 1, alice)
	in(ctx, 2, alice)
	in(ctx, -10, bob)

	got := new(metricdata.Aggregation)
	out(got)
	fAlice := attribute.NewSet(userAlice)
	fBob := attribute.NewSet(userBob)
	metricdatatest.AssertAggregationsEqual(t, metricdata.Sum[int64]{
		IsMonotonic: false,
		Temporality: metricdata.DeltaTemporality,
		DataPoints: []metricdata.DataPoint[int64]{
			{
				Attributes: fAlice,
				StartTime:  staticTime,
				Time:       staticTime,
				Value:      4,
				/*
					Exemplars: []metricdata.Exemplar[int64]{{
						FilteredAttributes: []attribute.KeyValue{adminTrue},
						Time:               staticTime,
						Value:              2,
					}},
				*/
			},
			{
				Attributes: fBob,
				StartTime:  staticTime,
				Time:       staticTime,
				Value:      -11,
			},
		},
	}, *got)

	/*
		a.Aggregate(ctx, 10, alice)
		a.Aggregate(ctx, 3, bob)

		got = a.Aggregation()
		assert.Equal(t, SumAggregation[int64]{
			IsMonotonic: false,
			DataPoints: []DataPoint[int64]{
				{Attributes: fAlice, Value: 10},
				{Attributes: fBob, Value: 3},
			},
		}, got)
	*/
}

/*
func TestDeltaSum(t *testing.T) {
	a := Delta(SumBuilder[int64](false))

	alice := attribute.NewSet(userAlice, adminTrue)
	bob := attribute.NewSet(userBob, adminFalse)

	a.Record(1, alice)
	a.Record(-1, bob)
	a.Record(1, alice)
	a.Record(2, alice)
	a.Record(-10, bob)

	got := a.Aggregate(NewExemplarReservoir(NoExemplars[int64]()))
	assert.Equal(t, SumAggregation[int64]{
		IsMonotonic: false,
		DataPoints: []DataPoint[int64]{
			{Attributes: alice, Value: 4},
			{Attributes: bob, Value: -11},
		},
	}, got)

	a.Record(10, alice)
	a.Record(3, bob)

	got = a.Aggregate(NewExemplarReservoir(NoExemplars[int64]()))
	assert.Equal(t, SumAggregation[int64]{
		IsMonotonic: false,
		DataPoints: []DataPoint[int64]{
			{Attributes: alice, Value: 10},
			{Attributes: bob, Value: 3},
		},
	}, got)
}

func TestFilteredDeltaSum(t *testing.T) {
	userOnly := func(kv attribute.KeyValue) bool {
		return kv.Key == attribute.Key(keyUser)
	}
	a := Delta(Filter(SumBuilder[int64](false), userOnly))

	alice := attribute.NewSet(userAlice, adminTrue)
	bob := attribute.NewSet(userBob, adminFalse)

	a.Record(1, alice)
	a.Record(-1, bob)
	a.Record(1, alice)
	a.Record(2, alice)
	a.Record(-10, bob)

	got := a.Aggregate(NewExemplarReservoir(NoExemplars[int64]()))
	fAlice := attribute.NewSet(userAlice)
	fBob := attribute.NewSet(userBob)
	assert.Equal(t, SumAggregation[int64]{
		IsMonotonic: false,
		DataPoints: []DataPoint[int64]{
			{Attributes: fAlice, Value: 4},
			{Attributes: fBob, Value: -11},
		},
	}, got)

	a.Record(10, alice)
	a.Record(3, bob)

	got = a.Aggregate(NewExemplarReservoir(NoExemplars[int64]()))
	assert.Equal(t, SumAggregation[int64]{
		IsMonotonic: false,
		DataPoints: []DataPoint[int64]{
			{Attributes: fAlice, Value: 10},
			{Attributes: fBob, Value: 3},
		},
	}, got)
}

func TestFilteredDeltaLastValue(t *testing.T) {
	a := Delta(LastValueBuilder[int64]())

	alice := attribute.NewSet(userAlice, adminTrue)
	bob := attribute.NewSet(userBob, adminFalse)

	a.Record(1, alice)
	a.Record(-1, bob)
	a.Record(1, alice)
	a.Record(2, alice)
	a.Record(-10, bob)

	got := a.Aggregate(NewExemplarReservoir(NoExemplars[int64]()))
	assert.Equal(t, GaugeAggregation[int64]{
		DataPoints: []DataPoint[int64]{
			{Attributes: alice, Value: 2},
			{Attributes: bob, Value: -10},
		},
	}, got)

	a.Record(10, alice)

	got = a.Aggregate(NewExemplarReservoir(NoExemplars[int64]()))
	assert.Equal(t, GaugeAggregation[int64]{
		DataPoints: []DataPoint[int64]{
			{Attributes: alice, Value: 10},
		},
	}, got)
}
*/

var bmarkResults metricdata.Aggregation

func benchmarkAggregatorN[N int64 | float64](b *testing.B, factory func() (Input[N], Output), count int) {
	ctx := context.Background()
	attrs := make([]attribute.Set, count)
	for i := range attrs {
		attrs[i] = attribute.NewSet(attribute.Int("value", i))
	}

	b.Run("Aggregate", func(b *testing.B) {
		got := &bmarkResults
		in, out := factory()
		b.ReportAllocs()
		b.ResetTimer()

		for n := 0; n < b.N; n++ {
			for _, attr := range attrs {
				in(ctx, 1, attr)
			}
		}

		out(got)
	})

	b.Run("Aggregations", func(b *testing.B) {
		outs := make([]Output, b.N)
		for n := range outs {
			in, out := factory()
			for _, attr := range attrs {
				in(ctx, 1, attr)
			}
			outs[n] = out
		}

		got := &bmarkResults
		b.ReportAllocs()
		b.ResetTimer()

		for n := 0; n < b.N; n++ {
			outs[n](got)
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

func BenchmarkSum(b *testing.B) {
	b.Run("Int64", benchmarkSum[int64])
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
		b := Builder[N]{
			Temporality: metricdata.DeltaTemporality,
			Filter: func(kv attribute.KeyValue) bool {
				return kv.Key == attribute.Key(keyUser)
			},
		}
		return b.Sum(false)
	}
	b.Run("Filtered", benchmarkAggregator(factory))
}
