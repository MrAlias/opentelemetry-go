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

package metric // import "go.opentelemetry.io/otel/sdk/metric"

import (
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel/metric/unit"
	"go.opentelemetry.io/otel/sdk/metric/internal"
	"go.opentelemetry.io/otel/sdk/metric/view"
)

var (
	errInstConflict       = errors.New("instrument already exists")
	errInstConflictScope  = fmt.Errorf("%w: scope conflict", errInstConflict)
	errInstConflictDesc   = fmt.Errorf("%w: description conflict", errInstConflict)
	errInstConflictAgg    = fmt.Errorf("%w: data type conflict", errInstConflict)
	errInstConflictUnit   = fmt.Errorf("%w: unit conflict", errInstConflict)
	errInstConflictNumber = fmt.Errorf("%w: number type conflict", errInstConflict)
)

// cache is a locking storage used to quickly return already computed values.
//
// The zero value of a cache is empty and ready to use.
//
// A cache must not be copied after first use.
//
// All methods of a cache are safe to call concurrently.
type cache[K comparable, V any] struct {
	sync.Mutex
	data map[K]V
}

// Lookup returns the value stored in the cache with the accociated key if it
// exists. Otherwise, f is called and its returned value is set in the cache
// for key and returned.
//
// Lookup is safe to call concurrently. It will hold the cache lock, so f
// should not block excessively.
func (c *cache[K, V]) Lookup(key K, f func() V) V {
	c.Lock()
	defer c.Unlock()

	if c.data == nil {
		val := f()
		c.data = map[K]V{key: val}
		return val
	}
	if v, ok := c.data[key]; ok {
		return v
	}
	val := f()
	c.data[key] = val
	return val
}

// aggCache is a cache for instrument Aggregators.
type aggCache[N int64 | float64] struct {
	cache *cache[string, any]
}

// newAggCache returns a new aggCache that uses c as the underlying cache. If c
// is nil, a new empty cache will be used.
func newAggCache[N int64 | float64](c *cache[string, any]) aggCache[N] {
	if c == nil {
		c = &cache[string, any]{}
	}
	return aggCache[N]{cache: c}
}

// Lookup returns the Aggregator and error for a cached instrument if it exist
// in the cache. Otherwise, f is called and its returned value is set in the
// cache and returned.
//
// If an instrument has been stored in the cache for a different N, an error is
// returned describing the conflict with a nil Aggregator.
//
// If an instrument has been stored in the cache with a different description,
// scope, aggregation data type, or unit, an error is returned describing the
// conflict along with the originally stored Aggregator.
//
// Lookup is safe to call concurrently.
func (c aggCache[N]) Lookup(inst view.Instrument, u unit.Unit, f func() (internal.Aggregator[N], error)) (agg internal.Aggregator[N], err error) {
	vAny := c.cache.Lookup(inst.Name, func() any {
		a, err := f()
		return aggVal[N]{
			Instrument: inst,
			Unit:       u,
			Aggregator: a,
			Err:        err,
		}
	})

	switch v := vAny.(type) {
	case aggVal[N]:
		agg = v.Aggregator
		err = v.conflict(inst, u)
		if err == nil {
			err = v.Err
		}
	default:
		err = errInstConflictNumber
	}
	return agg, err
}

// aggVal is the cached value of an aggCache.
type aggVal[N int64 | float64] struct {
	view.Instrument
	Unit       unit.Unit
	Aggregator internal.Aggregator[N]
	Err        error
}

// conflict returns an error describing any conflict the inst and u have with
// v. If both describe the same instrument, and are compatible, nil is
// returned.
func (v aggVal[N]) conflict(inst view.Instrument, u unit.Unit) error {
	// Assume name is already equal based on the cache lookup.
	switch false {
	case v.Scope == inst.Scope:
		return errInstConflictScope
	case v.Description == inst.Description:
		return errInstConflictDesc
	case v.Unit == u:
		return errInstConflictUnit
		// TODO: Enable Aggregation comparison according to the identifying
		// properties of the metric data-model.
		//case i.Aggregation == inst.Aggregation:
		//	return errInstConflictAgg
	}
	return nil
}
