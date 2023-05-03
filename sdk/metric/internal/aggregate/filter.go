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

type mapper[T any] func([]T) []T

// TODO: generalize with filterHistogramDataPoint
func filterDataPoints[N int64 | float64](f attribute.Filter, merge fold[metricdata.DataPoint[N]]) mapper[metricdata.DataPoint[N]] {
	return func(data []metricdata.DataPoint[N]) []metricdata.DataPoint[N] {
		index := make(map[attribute.Set]int)
		var i int
		for _, d := range data {
			filtered, dropped := d.Attributes.Filter(f)
			if len(dropped) > 0 {
				d.Exemplars = setDropped(dropped, d.Exemplars)
				d.Attributes = filtered
			}

			if idx, ok := index[filtered]; ok {
				data[idx] = merge(data[idx], d)
				data = append(data[:i], data[i+1:]...)
			} else {
				index[filtered] = i
				data[i] = d
				i++
			}
		}
		return data[:i]
	}
}

func filterHistogramDataPoint[N int64 | float64](f attribute.Filter, merge fold[metricdata.HistogramDataPoint[N]]) mapper[metricdata.HistogramDataPoint[N]] {
	return func(data []metricdata.HistogramDataPoint[N]) []metricdata.HistogramDataPoint[N] {
		index := make(map[attribute.Set]int)
		var i int
		for _, d := range data {
			filtered, dropped := d.Attributes.Filter(f)
			if len(dropped) > 0 {
				d.Exemplars = setDropped(dropped, d.Exemplars)
				d.Attributes = filtered
			}

			if idx, ok := index[filtered]; ok {
				data[idx] = merge(data[idx], d)
				data = append(data[:i], data[i+1:]...)
			} else {
				index[filtered] = i
				data[i] = d
				i++
			}
		}
		return data[:i]
	}
}

func setDropped[N int64 | float64](dropped []attribute.KeyValue, exemplar []metricdata.Exemplar[N]) []metricdata.Exemplar[N] {
	for _, e := range exemplar {
		e.FilteredAttributes = append(e.FilteredAttributes, dropped...)
	}
	return exemplar
}
