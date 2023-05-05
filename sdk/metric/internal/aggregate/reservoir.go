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
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/internal/exemplar"
)

type reservoir[N int64 | float64, T any] struct {
	sync.Mutex
	res exemplar.Reservoir[N]
	// collect is either Collect or Flush from res.
	collect  func() map[attribute.Set][]exemplar.Measurement[N]
	accessor accessor[N, T]
}

func newDeltaReservoir[N int64 | float64, T any](r exemplar.Reservoir[N], a accessor[N, T]) *reservoir[N, T] {
	if r == nil {
		return nil
	}
	return &reservoir[N, T]{
		res:      r,
		collect:  r.Flush,
		accessor: a,
	}
}

func newCumulativeReservoir[N int64 | float64, T any](r exemplar.Reservoir[N], a accessor[N, T]) *reservoir[N, T] {
	if r == nil {
		return nil
	}
	return &reservoir[N, T]{
		res:      r,
		collect:  r.Collect,
		accessor: a,
	}
}

func (r *reservoir[N, T]) offer(ctx context.Context, t time.Time, n N, attr attribute.Set) {
	r.Lock()
	defer r.Unlock()
	r.res.Offer(ctx, t, n, attr)
}

func (r *reservoir[N, T]) exemplars(iter *iterator[T]) {
	r.Lock()
	exemplars := r.collect()
	r.Unlock()

	item := r.accessor
	for iter.next() {
		item.wrap(iter.get())
		meas, ok := exemplars[item.attr()]
		if !ok {
			continue
		}

		dest := item.exemplars(len(meas))
		for i, m := range meas {
			dest[i] = m.Exemplar()
		}
		iter.set(item.unwrap())
	}
}
