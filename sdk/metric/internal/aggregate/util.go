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
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// now is used to return the current local time while allowing tests to
// override the default time.Now function.
var now = time.Now

type iterator[T any] struct {
	data []T
	idx  int
}

func newIterator[T any](data []T) *iterator[T] {
	return &iterator[T]{data: data, idx: -1}
}

func (i *iterator[T]) next() bool {
	i.idx++
	return i.idx < len(i.data)
}

func (i *iterator[T]) get() T { return i.data[i.idx] }

func (i *iterator[T]) getAt(idx int) T { return i.data[idx] }

func (i *iterator[T]) set(t T) { i.data[i.idx] = t }

func (i *iterator[T]) setAt(idx int, t T) { i.data[idx] = t }

func (i *iterator[T]) delete() {
	i.data = append(i.data[:i.idx], i.data[i.idx+1:]...)
	i.idx--
}

func (i *iterator[T]) index() int { return i.idx }

func (i *iterator[T]) slice() []T { return i.data }

// accessor wraps a metric data type to provide unified access to its fields.
type accessor[N int64 | float64, T any] interface {
	wrap(T)
	unwrap() T

	attr() attribute.Set
	filter(attribute.Filter) attribute.Set

	// Exemplars returns the slice of underlying Exemplars that is ensured to
	// have capacity and length n so values can be assigned to it.
	exemplars(n int) []metricdata.Exemplar[N]
}

type dPt[N int64 | float64] struct {
	data metricdata.DataPoint[N]
}

func (d *dPt[N]) wrap(data metricdata.DataPoint[N]) { d.data = data }

func (d *dPt[N]) unwrap() metricdata.DataPoint[N] { return d.data }

func (d *dPt[N]) attr() attribute.Set { return d.data.Attributes }

func (d *dPt[N]) filter(f attribute.Filter) attribute.Set {
	fltr, drop := d.data.Attributes.Filter(f)
	if len(drop) > 0 {
		d.data.Exemplars = setDropped(drop, d.data.Exemplars)
		d.data.Attributes = fltr
	}
	return fltr
}

func (d *dPt[N]) exemplars(n int) []metricdata.Exemplar[N] {
	d.data.Exemplars = minCap(d.data.Exemplars, n)
	return d.data.Exemplars
}

type histDPt[N int64 | float64] struct {
	data metricdata.HistogramDataPoint[N]
}

func (d *histDPt[N]) wrap(data metricdata.HistogramDataPoint[N]) { d.data = data }

func (d *histDPt[N]) unwrap() metricdata.HistogramDataPoint[N] { return d.data }

func (d *histDPt[N]) attr() attribute.Set { return d.data.Attributes }

func (d *histDPt[N]) filter(f attribute.Filter) attribute.Set {
	fltr, drop := d.data.Attributes.Filter(f)
	if len(drop) > 0 {
		d.data.Exemplars = setDropped(drop, d.data.Exemplars)
		d.data.Attributes = fltr
	}
	return fltr
}

func (d *histDPt[N]) exemplars(n int) []metricdata.Exemplar[N] {
	d.data.Exemplars = minCap(d.data.Exemplars, n)
	return d.data.Exemplars
}

func minCapEmpty[T any](v []T, n int) []T {
	if cap(v) < n {
		return make([]T, 0, n)
	}
	return v[:0]
}

func minCap[T any](v []T, n int) []T {
	if cap(v) < n {
		return make([]T, n)
	}
	return v[:n]
}

func setDropped[N int64 | float64](dropped []attribute.KeyValue, exemplar []metricdata.Exemplar[N]) []metricdata.Exemplar[N] {
	for i, e := range exemplar {
		e.FilteredAttributes = append(e.FilteredAttributes, dropped...)
		exemplar[i] = e
	}
	return exemplar
}
