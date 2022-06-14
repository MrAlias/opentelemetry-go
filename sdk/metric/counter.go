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

//go:build go1.18
// +build go1.18

package metric

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument"
)

// counter is an instrument used to record increasing values.
type counter[N int64 | float64] struct {
	instrument.Synchronous

	name string
	opts []instrument.Option
}

func newCounter[N int64 | float64](name string, opts []instrument.Option) *counter[N] {
	return &counter[N]{
		name: name,
		opts: opts,
	}
}

// Add records a positive change to the counter.
func (c counter[N]) Add(ctx context.Context, incr N, attrs ...attribute.KeyValue) {
	if incr <= 0 {
		// TODO: log an error.
	}
	// TODO: store value.
}

// upDownCounter is an instrument used to record changes of a value.
type upDownCounter[N int64 | float64] struct {
	instrument.Synchronous

	name string
	opts []instrument.Option
}

func newUpDownCounter[N int64 | float64](name string, opts []instrument.Option) *upDownCounter[N] {
	return &upDownCounter[N]{
		name: name,
		opts: opts,
	}
}

// Add records a change to the upDownCounter.
func (c upDownCounter[N]) Add(ctx context.Context, incr N, attrs ...attribute.KeyValue) {
	// TODO: store value.
}
