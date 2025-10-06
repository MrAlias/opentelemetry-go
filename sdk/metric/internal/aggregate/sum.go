// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregate // import "go.opentelemetry.io/otel/sdk/metric/internal/aggregate"

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type sumValue[N int64 | float64] struct {
	n     atomicCounter[N]
	res   FilteredExemplarReservoir[N]
	attrs attribute.Set
}

// valueMap is the storage for sums.
type valueMap[N int64 | float64] struct {
	newRes   func(attribute.Set) FilteredExemplarReservoir[N]
	aggLimit int

	// cumulative sums do not reset values during collection, so in that case
	// clearValuesOnCollection is false, hcwg is unused, and only values[0]
	// and len[0] are used. All other aggregations reset on collection, so we
	// use hcwg to swap between the hot and cold maps and len so measurements
	// can continue without blocking on collection.
	//
	// see hotColdWaitGroup for how this works.
	clearValuesOnCollection bool
	hcwg                    hotColdWaitGroup
	values                  [2]atomicMap[N]
}

type atomicMap[N int64 | float64] interface {
	Len() int
	LoadOrStore(key attribute.Distinct, value *sumValue[N]) (actual *sumValue[N], loaded bool)
	Range(f func(key attribute.Distinct, value *sumValue[N]) bool)
	Clear()
}

func newValueMap[N int64 | float64](
	limit int,
	r func(attribute.Set) FilteredExemplarReservoir[N],
	clearValuesOnCollection bool,
) *valueMap[N] {
	if limit < 0 {
		limit = 0
	}
	var values [2]atomicMap[N]
	// Use sync.Map if there is no limit, otherwise use cappedMap.
	if limit == 0 {
		values[0] = newAtomicSyncMap[N]()
		values[1] = newAtomicSyncMap[N]()
	} else {
		values[0] = newCappedMap[N](limit, r)
		values[1] = newCappedMap[N](limit, r)
	}
	return &valueMap[N]{
		newRes:                  r,
		aggLimit:                limit,
		clearValuesOnCollection: clearValuesOnCollection,
		values:                  values,
	}
}

func (s *valueMap[N]) measure(ctx context.Context, value N, fltrAttr attribute.Set, droppedAttr []attribute.KeyValue) {
	hotIdx := uint64(0)
	if s.clearValuesOnCollection {
		hotIdx = s.hcwg.start()
		defer s.hcwg.done(hotIdx)
	}
	v, _ := s.values[hotIdx].LoadOrStore(fltrAttr.Equivalent(), &sumValue[N]{
		res:   s.newRes(fltrAttr),
		attrs: fltrAttr,
	})
	v.n.add(value)
	// It is possible for collection to race with measurement and observe the
	// exemplar in the batch of metrics after the add() for cumulative sums.
	// This is an accepted tradeoff to avoid locking during measurement.
	v.res.Offer(ctx, value, droppedAttr)
}

// newSum returns an aggregator that summarizes a set of measurements as their
// arithmetic sum. Each sum is scoped by attributes and the aggregation cycle
// the measurements were made in.
func newSum[N int64 | float64](
	monotonic bool,
	temporality metricdata.Temporality,
	limit int,
	r func(attribute.Set) FilteredExemplarReservoir[N],
) *sum[N] {
	clearValuesOnCollection := temporality == metricdata.DeltaTemporality
	return &sum[N]{
		valueMap:  newValueMap[N](limit, r, clearValuesOnCollection),
		monotonic: monotonic,
		start:     now(),
	}
}

// sum summarizes a set of measurements made as their arithmetic sum.
type sum[N int64 | float64] struct {
	*valueMap[N]

	monotonic bool
	start     time.Time
}

func (s *sum[N]) delta(
	dest *metricdata.Aggregation, //nolint:gocritic // The pointer is needed for the ComputeAggregation interface
) int {
	t := now()

	// If *dest is not a metricdata.Sum, memory reuse is missed. In that case,
	// use the zero-value sData and hope for better alignment next cycle.
	sData, _ := (*dest).(metricdata.Sum[N])
	sData.Temporality = metricdata.DeltaTemporality
	sData.IsMonotonic = s.monotonic

	// delta always clears values on collection
	readIdx := s.hcwg.swapHotAndWait()
	// The len will not change while we iterate over values, since we waited
	// for all writes to finish to the cold values and len.
	n := s.values[readIdx].Len()
	dPts := reset(sData.DataPoints, n, n)

	var i int
	s.values[readIdx].Range(func(_ attribute.Distinct, val *sumValue[N]) bool {
		collectExemplars(&dPts[i].Exemplars, val.res.Collect)
		dPts[i].Attributes = val.attrs
		dPts[i].StartTime = s.start
		dPts[i].Time = t
		dPts[i].Value = val.n.load()
		i++
		return true
	})
	s.values[readIdx].Clear()
	// The delta collection cycle resets.
	s.start = t

	sData.DataPoints = dPts
	*dest = sData

	return i
}

func (s *sum[N]) cumulative(
	dest *metricdata.Aggregation, //nolint:gocritic // The pointer is needed for the ComputeAggregation interface
) int {
	t := now()

	// If *dest is not a metricdata.Sum, memory reuse is missed. In that case,
	// use the zero-value sData and hope for better alignment next cycle.
	sData, _ := (*dest).(metricdata.Sum[N])
	sData.Temporality = metricdata.CumulativeTemporality
	sData.IsMonotonic = s.monotonic

	readIdx := 0
	// Values are being concurrently written while we iterate, so only use the
	// current length for capacity.
	dPts := reset(sData.DataPoints, 0, s.values[readIdx].Len())

	var i int
	s.values[readIdx].Range(func(_ attribute.Distinct, val *sumValue[N]) bool {
		newPt := metricdata.DataPoint[N]{
			Attributes: val.attrs,
			StartTime:  s.start,
			Time:       t,
			Value:      val.n.load(),
		}
		collectExemplars(&newPt.Exemplars, val.res.Collect)
		dPts = append(dPts, newPt)
		// TODO (#3006): This will use an unbounded amount of memory if there
		// are unbounded number of attribute sets being aggregated. Attribute
		// sets that become "stale" need to be forgotten so this will not
		// overload the system.
		i++
		return true
	})

	sData.DataPoints = dPts
	*dest = sData

	return i
}

// newPrecomputedSum returns an aggregator that summarizes a set of
// observations as their arithmetic sum. Each sum is scoped by attributes and
// the aggregation cycle the measurements were made in.
func newPrecomputedSum[N int64 | float64](
	monotonic bool,
	limit int,
	r func(attribute.Set) FilteredExemplarReservoir[N],
) *precomputedSum[N] {
	return &precomputedSum[N]{
		valueMap:  newValueMap[N](limit, r, true),
		monotonic: monotonic,
		start:     now(),
	}
}

// precomputedSum summarizes a set of observations as their arithmetic sum.
type precomputedSum[N int64 | float64] struct {
	*valueMap[N]

	monotonic bool
	start     time.Time

	reported map[any]N
}

func (s *precomputedSum[N]) delta(
	dest *metricdata.Aggregation, //nolint:gocritic // The pointer is needed for the ComputeAggregation interface
) int {
	t := now()
	newReported := make(map[any]N)

	// If *dest is not a metricdata.Sum, memory reuse is missed. In that case,
	// use the zero-value sData and hope for better alignment next cycle.
	sData, _ := (*dest).(metricdata.Sum[N])
	sData.Temporality = metricdata.DeltaTemporality
	sData.IsMonotonic = s.monotonic

	// delta always clears values on collection
	readIdx := s.hcwg.swapHotAndWait()
	// The len will not change while we iterate over values, since we waited
	// for all writes to finish to the cold values and len.
	n := s.values[readIdx].Len()
	dPts := reset(sData.DataPoints, n, n)

	var i int
	s.values[readIdx].Range(func(key attribute.Distinct, val *sumValue[N]) bool {
		n := val.n.load()

		delta := n - s.reported[key]
		collectExemplars(&dPts[i].Exemplars, val.res.Collect)
		dPts[i].Attributes = val.attrs
		dPts[i].StartTime = s.start
		dPts[i].Time = t
		dPts[i].Value = delta
		newReported[key] = n
		i++
		return true
	})
	s.values[readIdx].Clear()
	s.reported = newReported
	// The delta collection cycle resets.
	s.start = t

	sData.DataPoints = dPts
	*dest = sData

	return i
}

func (s *precomputedSum[N]) cumulative(
	dest *metricdata.Aggregation, //nolint:gocritic // The pointer is needed for the ComputeAggregation interface
) int {
	t := now()

	// If *dest is not a metricdata.Sum, memory reuse is missed. In that case,
	// use the zero-value sData and hope for better alignment next cycle.
	sData, _ := (*dest).(metricdata.Sum[N])
	sData.Temporality = metricdata.CumulativeTemporality
	sData.IsMonotonic = s.monotonic

	// cumulative precomputed always clears values on collection
	readIdx := s.hcwg.swapHotAndWait()
	// The len will not change while we iterate over values, since we waited
	// for all writes to finish to the cold values and len.
	n := s.values[readIdx].Len()
	dPts := reset(sData.DataPoints, n, n)

	var i int
	s.values[readIdx].Range(func(_ attribute.Distinct, val *sumValue[N]) bool {
		collectExemplars(&dPts[i].Exemplars, val.res.Collect)
		dPts[i].Attributes = val.attrs
		dPts[i].StartTime = s.start
		dPts[i].Time = t
		dPts[i].Value = val.n.load()
		i++
		return true
	})
	s.values[readIdx].Clear()

	sData.DataPoints = dPts
	*dest = sData

	return i
}
