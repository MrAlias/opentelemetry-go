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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/unit"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/internal"
	"go.opentelemetry.io/otel/sdk/metric/view"
)

func TestCache(t *testing.T) {
	k0, k1 := "one", "two"
	v0, v1 := 1, 2

	c := cache[string, int]{}

	var got int
	require.NotPanics(t, func() {
		got = c.Lookup(k0, func() int { return v0 })
	}, "zero-value cache panics on Lookup")
	assert.Equal(t, v0, got, "zero-value cache did not return fallback")

	assert.Equal(t, v0, c.Lookup(k0, func() int { return v1 }), "existing key")

	assert.Equal(t, v1, c.Lookup(k1, func() int { return v1 }), "non-existing key")
}

func TestAggCacheNumberConflict(t *testing.T) {
	c := cache[string, any]{}

	inst := view.Instrument{
		Scope:       instrumentation.Scope{Name: "scope name"},
		Name:        "name",
		Description: "description",
	}
	u := unit.Dimensionless
	aggs := internal.NewCumulativeSum[int64](true)

	instCachI := newAggCache[int64](&c)
	gotI, err := instCachI.Lookup(inst, u, func() (internal.Aggregator[int64], error) {
		return aggs, nil
	})
	require.NoError(t, err)
	require.Equal(t, aggs, gotI)

	instCachF := newAggCache[float64](&c)
	gotF, err := instCachF.Lookup(inst, u, func() (internal.Aggregator[float64], error) {
		return internal.NewCumulativeSum[float64](true), nil
	})
	assert.ErrorIs(t, err, errInstConflictNumber)
	assert.Nil(t, gotF, "cache conflict should not return a value")
}
