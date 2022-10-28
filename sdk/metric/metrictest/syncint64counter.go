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

package metrictest // import "go.opentelemetry.io/otel/sdk/metric/metrictest"

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
)

func testSyncInt64Counter(f Factory) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("Default", testSyncInt64CounterDefault(f))
		t.Run("Delta", testSyncInt64CounterDelta(f))
	}
}

func testSyncInt64CounterDefault(f Factory) func(*testing.T) {
	const (
		goroutines = 30
		cycles     = 3
		name       = "logins"
		desc       = "Number of user logins"
		u          = unit.Dimensionless
	)

	return func(t *testing.T) {
		mp, g := f(defualtConf)
		m := mp.Meter(
			pkgName,
			metric.WithInstrumentationVersion(version),
			metric.WithSchemaURL(schemaURL),
		)
		logins, err := m.SyncInt64().Counter(
			name,
			instrument.WithDescription(desc),
			instrument.WithUnit(u),
		)
		require.NoError(t, err)

		for cycle := 0; cycle < cycles; cycle++ {
			ctx := context.Background()
			var wg sync.WaitGroup
			for n := 0; n < goroutines; n++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					logins.Add(ctx, 0, bob...) // No-op.
					logins.Add(ctx, 1, alice...)
					logins.Add(ctx, 2, bob...)
				}()
			}
			wg.Wait()

			require.NoError(t, g.Gather(ctx))
		}

		got := g.Storage().dump()
		require.Len(t, got, cycles, "wrong number of collection cycles")

		want := metricdata.ResourceMetrics{
			Resource: defaultRes,
			ScopeMetrics: []metricdata.ScopeMetrics{{
				Scope: defualtScope,
				Metrics: []metricdata.Metrics{{
					Name:        name,
					Description: desc,
					Unit:        u,
					Data: metricdata.Sum[int64]{
						Temporality: metricdata.CumulativeTemporality,
						IsMonotonic: true,
						DataPoints: []metricdata.DataPoint[int64]{{
							Attributes: attribute.NewSet(alice...),
						}, {
							Attributes: attribute.NewSet(bob...),
						}},
					},
				}},
			}},
		}
		for c := 0; c < cycles; c++ {
			dpts := want.ScopeMetrics[0].Metrics[0].Data.(metricdata.Sum[int64]).DataPoints
			dpts[0].Value = int64(1 * goroutines * (c + 1))
			dpts[1].Value = int64(2 * goroutines * (c + 1))
			metricdatatest.AssertEqual(t, got[c], want, metricdatatest.IgnoreTimestamp())
		}
	}
}

func testSyncInt64CounterDelta(f Factory) func(*testing.T) {
	const (
		goroutines = 30
		name       = "logins"
		desc       = "Number of user logins"
		u          = unit.Dimensionless
	)

	return func(t *testing.T) {
		conf := defualtConf
		conf.Temporality = metricdata.DeltaTemporality
		mp, coll := f(conf)
		m := mp.Meter(
			pkgName,
			metric.WithInstrumentationVersion(version),
			metric.WithSchemaURL(schemaURL),
		)
		c, err := m.SyncInt64().Counter(
			name,
			instrument.WithDescription(desc),
			instrument.WithUnit(u),
		)
		require.NoError(t, err)

		ctx := context.Background()
		var wg sync.WaitGroup
		for n := 0; n < goroutines; n++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c.Add(ctx, 0, bob...) // No-op.
				c.Add(ctx, 1, alice...)
				c.Add(ctx, 2, bob...)
			}()
		}
		wg.Wait()

		if mps, ok := mp.(interface{ Shutdown(context.Context) error }); ok {
			require.NoError(t, mps.Shutdown(ctx))
		}

		got := coll.Storage().dump()
		require.Len(t, got, 1, "one export expected")
		metricdatatest.AssertEqual(
			t,
			metricdata.ResourceMetrics{
				Resource: defaultRes,
				ScopeMetrics: []metricdata.ScopeMetrics{{
					Scope: defualtScope,
					Metrics: []metricdata.Metrics{{
						Name:        name,
						Description: desc,
						Unit:        u,
						Data: metricdata.Sum[int64]{
							Temporality: metricdata.DeltaTemporality,
							IsMonotonic: true,
							DataPoints: []metricdata.DataPoint[int64]{{
								Attributes: attribute.NewSet(alice...),
								Value:      goroutines,
							}, {
								Attributes: attribute.NewSet(bob...),
								Value:      2 * goroutines,
							}},
						},
					}},
				}},
			},
			got[0],
			metricdatatest.IgnoreTimestamp(),
		)
	}
}
