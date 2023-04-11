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

package global // import "go.opentelemetry.io/otel/internal/global"

import (
	"context"
	"sync/atomic"

	"go.opentelemetry.io/otel/metric/embedded"
	"go.opentelemetry.io/otel/metric/instrument"
)

var (
	_ unwrapper = (*observableCounter[int64])(nil)
	_ unwrapper = (*observableCounter[float64])(nil)
	_ unwrapper = (*observableUpDownCounter[int64])(nil)
	_ unwrapper = (*observableUpDownCounter[float64])(nil)
	_ unwrapper = (*observableGauge[int64])(nil)
	_ unwrapper = (*observableGauge[float64])(nil)

	_ instrument.Counter[int64]                   = (*counter[int64])(nil)
	_ instrument.Counter[float64]                 = (*counter[float64])(nil)
	_ instrument.UpDownCounter[int64]             = (*upDownCounter[int64])(nil)
	_ instrument.UpDownCounter[float64]           = (*upDownCounter[float64])(nil)
	_ instrument.Histogram[int64]                 = (*histogram[int64])(nil)
	_ instrument.Histogram[float64]               = (*histogram[float64])(nil)
	_ instrument.ObservableCounter[int64]         = (*observableCounter[int64])(nil)
	_ instrument.ObservableCounter[float64]       = (*observableCounter[float64])(nil)
	_ instrument.ObservableUpDownCounter[int64]   = (*observableUpDownCounter[int64])(nil)
	_ instrument.ObservableUpDownCounter[float64] = (*observableUpDownCounter[float64])(nil)
	_ instrument.ObservableGauge[int64]           = (*observableGauge[int64])(nil)
	_ instrument.ObservableGauge[float64]         = (*observableGauge[float64])(nil)
)

// unwrapper unwraps to return the underlying instrument implementation.
type unwrapper interface {
	Unwrap() instrument.Observable
}

type observableCounter[N int64 | float64] struct {
	embedded.ObservableCounter[N]
	instrument.ObservableT[N]
	atomic.Value

	name string
	opts []instrument.ObservableCounterOption[N]
}

func (i *observableCounter[N]) Unwrap() instrument.Observable {
	if ctr := i.Load(); ctr != nil {
		return ctr.(instrument.ObservableCounter[N])
	}
	return nil
}

type observableUpDownCounter[N int64 | float64] struct {
	embedded.ObservableUpDownCounter[N]
	instrument.ObservableT[N]
	atomic.Value

	name string
	opts []instrument.ObservableUpDownCounterOption[N]
}

func (i *observableUpDownCounter[N]) Unwrap() instrument.Observable {
	if ctr := i.Load(); ctr != nil {
		return ctr.(instrument.ObservableUpDownCounter[N])
	}
	return nil
}

type observableGauge[N int64 | float64] struct {
	embedded.ObservableGauge[N]
	instrument.ObservableT[N]
	atomic.Value

	name string
	opts []instrument.ObservableGaugeOption[N]
}

func (i *observableGauge[N]) Unwrap() instrument.Observable {
	if ctr := i.Load(); ctr != nil {
		return ctr.(instrument.ObservableGauge[N])
	}
	return nil
}

type counter[N int64 | float64] struct {
	embedded.Counter[N]
	atomic.Value

	name string
	opts []instrument.CounterOption[N]
}

func (i *counter[N]) Add(ctx context.Context, incr N, opts ...instrument.AddOption[N]) {
	if ctr := i.Load(); ctr != nil {
		ctr.(instrument.Counter[N]).Add(ctx, incr, opts...)
	}
}

type upDownCounter[N int64 | float64] struct {
	embedded.UpDownCounter[N]
	atomic.Value

	name string
	opts []instrument.UpDownCounterOption[N]
}

func (i *upDownCounter[N]) Add(ctx context.Context, incr N, opts ...instrument.AddOption[N]) {
	if ctr := i.Load(); ctr != nil {
		ctr.(instrument.UpDownCounter[N]).Add(ctx, incr, opts...)
	}
}

type histogram[N int64 | float64] struct {
	embedded.Histogram[N]
	atomic.Value

	name string
	opts []instrument.HistogramOption[N]
}

func (i *histogram[N]) Record(ctx context.Context, x N, opts ...instrument.RecordOption[N]) {
	if ctr := i.Load(); ctr != nil {
		ctr.(instrument.Histogram[N]).Record(ctx, x, opts...)
	}
}
