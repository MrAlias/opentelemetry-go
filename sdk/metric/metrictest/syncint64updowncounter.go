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
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
)

func testSyncInt64UpDownCounter(f Factory) func(*testing.T) {
	name := "priority"
	desc := "Importance of a user"
	u := unit.Unit("{priority}")

	return func(t *testing.T) {
		mp, coll := f(defualtConf)
		m := mp.Meter(
			pkgName,
			metric.WithInstrumentationVersion(version),
			metric.WithSchemaURL(schemaURL),
		)
		c, err := m.SyncInt64().UpDownCounter(
			name,
			instrument.WithDescription(desc),
			instrument.WithUnit(u),
		)
		require.NoError(t, err)

		ctx := context.Background()
		c.Add(ctx, 12, alice...)
		c.Add(ctx, -3, bob...)
		c.Add(ctx, 16, alice...)
		c.Add(ctx, 1, bob...)

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
							Temporality: metricdata.CumulativeTemporality,
							IsMonotonic: false,
							DataPoints: []metricdata.DataPoint[int64]{{
								Attributes: attribute.NewSet(alice...),
								Value:      28,
							}, {
								Attributes: attribute.NewSet(bob...),
								Value:      -2,
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
