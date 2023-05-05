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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type fold[T any] func(T, T) T

func filter[N int64 | float64, T any](f attribute.Filter, merge fold[T], item accessor[N, T]) func(iter *iterator[T]) []T {
	return func(iter *iterator[T]) []T {
		index := make(map[attribute.Set]int)
		for iter.next() {
			item.wrap(iter.get())

			fltr := item.filter(f)
			if idx, ok := index[fltr]; ok {
				iter.setAt(idx, merge(iter.getAt(idx), item.unwrap()))
				iter.delete()
			} else {
				index[fltr] = iter.index()
				iter.set(item.unwrap())
			}
		}
		return iter.slice()
	}
}

func filterDPtFn[N int64 | float64](f attribute.Filter, merge fold[metricdata.DataPoint[N]]) func([]metricdata.DataPoint[N]) []metricdata.DataPoint[N] {
	fltr := filter[N, metricdata.DataPoint[N]](f, merge, &dPt[N]{})
	return func(d []metricdata.DataPoint[N]) []metricdata.DataPoint[N] {
		return fltr(newIterator(d))
	}
}

func filterHistDPtFn[N int64 | float64](f attribute.Filter, merge fold[metricdata.HistogramDataPoint[N]]) func([]metricdata.HistogramDataPoint[N]) []metricdata.HistogramDataPoint[N] {
	fltr := filter[N, metricdata.HistogramDataPoint[N]](f, merge, &histDPt[N]{})
	return func(d []metricdata.HistogramDataPoint[N]) []metricdata.HistogramDataPoint[N] {
		return fltr(newIterator(d))
	}
}
