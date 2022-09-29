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
	"context"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/asyncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/asyncint64"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.opentelemetry.io/otel/sdk/instrumentation"
)

// meter handles the creation and coordination of all metric instruments. A
// meter represents a single instrumentation scope; all metric telemetry
// produced by an instrumentation scope will use metric instruments from a
// single meter.
type meter struct {
	// cache ensures no duplicate Aggregators are created from the same
	// instrument within the scope of all instruments this meter owns.
	//
	// If a user requests the exact same instrument twice, a providers cache
	// should handle the request with its stored value. However, if a new
	// instrument is create that has a view transform it into an instrument
	// that was already created, that previous instrument's Aggregator needs to
	// be returned if it is compatible.
	cache cache[string, any]

	pipes pipelines

	asyncInt64Provider   asyncInt64Provider
	asyncFloat64Provider asyncFloat64Provider
	syncInt64Provider    syncInt64Provider
	syncFloat64Provider  syncFloat64Provider
}

func newMeter(s instrumentation.Scope, p pipelines) *meter {
	m := &meter{pipes: p}
	m.asyncInt64Provider = newAsyncInt64Provider(s, p, &m.cache)
	m.asyncFloat64Provider = newAsyncFloat64Provider(s, p, &m.cache)
	m.syncInt64Provider = newSyncInt64Provider(s, p, &m.cache)
	m.syncFloat64Provider = newSyncFloat64Provider(s, p, &m.cache)
	return m
}

// Compile-time check meter implements metric.Meter.
var _ metric.Meter = (*meter)(nil)

// AsyncInt64 returns the asynchronous integer instrument provider.
func (m *meter) AsyncInt64() asyncint64.InstrumentProvider { return m.asyncInt64Provider }

// AsyncFloat64 returns the asynchronous floating-point instrument provider.
func (m *meter) AsyncFloat64() asyncfloat64.InstrumentProvider { return m.asyncFloat64Provider }

// SyncInt64 returns the synchronous integer instrument provider.
func (m *meter) SyncInt64() syncint64.InstrumentProvider { return m.syncInt64Provider }

// SyncFloat64 returns the synchronous floating-point instrument provider.
func (m *meter) SyncFloat64() syncfloat64.InstrumentProvider { return m.syncFloat64Provider }

// RegisterCallback registers the function f to be called when any of the insts
// Collect method is called.
func (m *meter) RegisterCallback(insts []instrument.Asynchronous, f func(context.Context)) error {
	m.pipes.registerCallback(f)
	return nil
}
