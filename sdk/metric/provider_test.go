// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metric

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metrictest"
	"go.opentelemetry.io/otel/sdk/metric/view"
)

type collector struct {
	Reader

	storage *metrictest.Storage
}

func (c *collector) Gather(ctx context.Context) error {
	_, err := c.Collect(ctx)
	return err
}

func (c *collector) Storage() *metrictest.Storage {
	return c.storage
}

func (c *collector) Collect(ctx context.Context) (metricdata.ResourceMetrics, error) {
	data, err := c.Reader.Collect(ctx)
	c.storage.Add(data)
	return data, err

}

func (c *collector) ForceFlush(ctx context.Context) error {
	_, err := c.Collect(ctx)
	if err != nil {
		return err
	}
	return c.Reader.ForceFlush(ctx)
}

func (c *collector) Shutdown(ctx context.Context) error {
	_, err := c.Collect(ctx)
	if err != nil {
		return err
	}
	return c.Reader.Shutdown(ctx)
}

func factory(c metrictest.Config) (metric.MeterProvider, metrictest.Gatherer) {
	reader := NewManualReader(
		WithTemporalitySelector(
			func(ik view.InstrumentKind) metricdata.Temporality {
				var undefTemp metricdata.Temporality
				if c.Temporality == undefTemp {
					return DefaultTemporalitySelector(ik)
				}
				return c.Temporality
			},
		),
		WithAggregationSelector(
			func(ik view.InstrumentKind) aggregation.Aggregation {
				if c.Aggregation == nil {
					return DefaultAggregationSelector(ik)
				}
				return c.Aggregation
			},
		),
	)
	coll := &collector{
		Reader:  reader,
		storage: metrictest.NewStorage(),
	}
	mp := NewMeterProvider(
		WithResource(c.Resource),
		WithReader(coll),
	)
	return mp, coll
}

func TestMetricSDKTestSuite(t *testing.T) {
	metrictest.Run(t, factory)
}

func TestMeterConcurrentSafe(t *testing.T) {
	const name = "TestMeterConcurrentSafe meter"
	mp := NewMeterProvider()

	go func() {
		_ = mp.Meter(name)
	}()

	_ = mp.Meter(name)
}

func TestForceFlushConcurrentSafe(t *testing.T) {
	mp := NewMeterProvider()

	go func() {
		_ = mp.ForceFlush(context.Background())
	}()

	_ = mp.ForceFlush(context.Background())
}

func TestShutdownConcurrentSafe(t *testing.T) {
	mp := NewMeterProvider()

	go func() {
		_ = mp.Shutdown(context.Background())
	}()

	_ = mp.Shutdown(context.Background())
}

func TestMeterDoesNotPanicForEmptyMeterProvider(t *testing.T) {
	mp := MeterProvider{}
	assert.NotPanics(t, func() { _ = mp.Meter("") })
}

func TestForceFlushDoesNotPanicForEmptyMeterProvider(t *testing.T) {
	mp := MeterProvider{}
	assert.NotPanics(t, func() { _ = mp.ForceFlush(context.Background()) })
}

func TestShutdownDoesNotPanicForEmptyMeterProvider(t *testing.T) {
	mp := MeterProvider{}
	assert.NotPanics(t, func() { _ = mp.Shutdown(context.Background()) })
}

func TestMeterProviderReturnsSameMeter(t *testing.T) {
	mp := MeterProvider{}
	mtr := mp.Meter("")

	assert.Same(t, mtr, mp.Meter(""))
	assert.NotSame(t, mtr, mp.Meter("diff"))
}
