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

package stdout // import "go.opentelemetry.io/otel/exporters/metric/stdout"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/kv"
	"go.opentelemetry.io/otel/api/label"
	"go.opentelemetry.io/otel/api/metric"
	export "go.opentelemetry.io/otel/sdk/export/metric"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/controller/push"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
)

type Exporter struct {
	config Config
}

var _ export.Exporter = &Exporter{}

// Config is the configuration to be used when initializing a stdout export.
type Config struct {
	// Writer is the destination.  If not set, os.Stdout is used.
	Writer io.Writer

	// PrettyPrint will pretty the json representation of the span,
	// making it print "pretty". Default is false.
	PrettyPrint bool

	// DoNotPrintTime suppresses timestamp printing.  This is
	// useful to create deterministic test conditions.
	DoNotPrintTime bool

	// Quantiles are the desired aggregation quantiles for distribution
	// summaries, used when the configured aggregator supports
	// quantiles.
	//
	// Note: this exporter is meant as a demonstration; a real
	// exporter may wish to configure quantiles on a per-metric
	// basis.
	Quantiles []float64

	// LabelEncoder encodes the labels
	LabelEncoder label.Encoder
}

type expoBatch struct {
	Timestamp *time.Time `json:"time,omitempty"`
	Updates   []expoLine `json:"updates"`
}

type expoLine struct {
	Name      string      `json:"name"`
	Min       interface{} `json:"min,omitempty"`
	Max       interface{} `json:"max,omitempty"`
	Sum       interface{} `json:"sum,omitempty"`
	Count     interface{} `json:"count,omitempty"`
	LastValue interface{} `json:"last,omitempty"`
	Points    []point     `json:"points,omitempty"`
	Quantiles []quantile  `json:"quantiles,omitempty"`
	Histogram []bucket    `json:"histogram,omitempty"`

	// Note: this is a pointer because omitempty doesn't work when time.IsZero()
	Timestamp *time.Time `json:"time,omitempty"`
}

type quantile struct {
	Q interface{} `json:"q"`
	V interface{} `json:"v"`
}

type boundary struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type bucket struct {
	boundary `json:"bounds"`
	Count    float64 `json:"count"`
}

type point struct {
	Value interface{} `json:"value"`
}

// NewRawExporter creates a stdout Exporter for use in a pipeline.
func NewRawExporter(config Config) (*Exporter, error) {
	if config.Writer == nil {
		config.Writer = os.Stdout
	}
	if config.Quantiles == nil {
		config.Quantiles = []float64{0.5, 0.9, 0.99}
	} else {
		for _, q := range config.Quantiles {
			if q < 0 || q > 1 {
				return nil, aggregation.ErrInvalidQuantile
			}
		}
	}
	if config.LabelEncoder == nil {
		config.LabelEncoder = label.DefaultEncoder()
	}
	return &Exporter{
		config: config,
	}, nil
}

// InstallNewPipeline instantiates a NewExportPipeline and registers it globally.
// Typically called as:
//
// 	pipeline, err := stdout.InstallNewPipeline(stdout.Config{...})
// 	if err != nil {
// 		...
// 	}
// 	defer pipeline.Stop()
// 	... Done
func InstallNewPipeline(config Config, options ...push.Option) (*push.Controller, error) {
	controller, err := NewExportPipeline(config, options...)
	if err != nil {
		return controller, err
	}
	global.SetMeterProvider(controller.Provider())
	return controller, err
}

// NewExportPipeline sets up a complete export pipeline with the
// recommended setup, chaining a NewRawExporter into the recommended
// selectors and processors.
func NewExportPipeline(config Config, options ...push.Option) (*push.Controller, error) {
	exporter, err := NewRawExporter(config)
	if err != nil {
		return nil, err
	}
	pusher := push.New(
		simple.NewWithExactDistribution(),
		exporter,
		options...,
	)
	pusher.Start()

	return pusher, nil
}

func (e *Exporter) ExportKindFor(*metric.Descriptor, aggregation.Kind) export.ExportKind {
	return export.PassThroughExporter
}

func (e *Exporter) Export(_ context.Context, checkpointSet export.CheckpointSet) error {
	var aggError error
	var batch expoBatch
	if !e.config.DoNotPrintTime {
		ts := time.Now()
		batch.Timestamp = &ts
	}
	aggError = checkpointSet.ForEach(e, func(record export.Record) error {
		desc := record.Descriptor()
		agg := record.Aggregation()
		kind := desc.NumberKind()
		encodedResource := record.Resource().Encoded(e.config.LabelEncoder)

		var instLabels []kv.KeyValue
		if name := desc.InstrumentationName(); name != "" {
			instLabels = append(instLabels, kv.String("instrumentation.name", name))
			if version := desc.InstrumentationVersion(); version != "" {
				instLabels = append(instLabels, kv.String("instrumentation.version", version))
			}
		}
		instSet := label.NewSet(instLabels...)
		encodedInstLabels := instSet.Encoded(e.config.LabelEncoder)

		var (
			expose expoLine
			err    error
			p      = parser{e.config, kind}
		)
		switch t := agg.(type) {
		case aggregation.Distribution:
			expose.Quantiles, expose.Min, expose.Max, expose.Sum, expose.Count, err = p.Distribution(t)
		case aggregation.Quantile:
			expose.Quantiles, err = p.Quantile(t)
		case aggregation.Histogram:
			expose.Sum, expose.Histogram, err = p.Histogram(t)
		case aggregation.MinMaxSumCount:
			expose.Min, expose.Max, expose.Sum, expose.Count, err = p.MinMaxSumCount(t)
		case aggregation.Min:
			expose.Min, err = p.Min(t)
		case aggregation.Max:
			expose.Max, err = p.Max(t)
		case aggregation.Sum:
			expose.Sum, err = p.Sum(t)
		case aggregation.Count:
			expose.Count, err = p.Count(t)
		case aggregation.Points:
			expose.Points, err = p.Points(t)
		case aggregation.LastValue:
			expose.LastValue, expose.Timestamp, err = p.LastValue(t)
		}
		if err != nil {
			return err
		}

		var encodedLabels string
		iter := record.Labels().Iter()
		if iter.Len() > 0 {
			encodedLabels = record.Labels().Encoded(e.config.LabelEncoder)
		}

		var sb strings.Builder

		sb.WriteString(desc.Name())

		if len(encodedLabels) > 0 || len(encodedResource) > 0 || len(encodedInstLabels) > 0 {
			sb.WriteRune('{')
			sb.WriteString(encodedResource)
			if len(encodedInstLabels) > 0 && len(encodedResource) > 0 {
				sb.WriteRune(',')
			}
			sb.WriteString(encodedInstLabels)
			if len(encodedLabels) > 0 && (len(encodedInstLabels) > 0 || len(encodedResource) > 0) {
				sb.WriteRune(',')
			}
			sb.WriteString(encodedLabels)
			sb.WriteRune('}')
		}

		expose.Name = sb.String()

		batch.Updates = append(batch.Updates, expose)
		return nil
	})

	var data []byte
	var err error
	if e.config.PrettyPrint {
		data, err = json.MarshalIndent(batch, "", "\t")
	} else {
		data, err = json.Marshal(batch)
	}

	if err == nil {
		fmt.Fprintln(e.config.Writer, string(data))
	} else {
		return err
	}

	return aggError
}

type parser struct {
	Config Config
	Kind   metric.NumberKind
}

func (p parser) nToI(n metric.Number, err error) (interface{}, error) {
	if err != nil {
		return nil, err
	}
	return n.AsInterface(p.Kind), nil
}

func (p parser) Min(m aggregation.Min) (interface{}, error) {
	return p.nToI(m.Min())
}

func (p parser) Max(m aggregation.Max) (interface{}, error) {
	return p.nToI(m.Max())
}

func (p parser) Sum(s aggregation.Sum) (interface{}, error) {
	return p.nToI(s.Sum())
}

func (p parser) Count(c aggregation.Count) (interface{}, error) {
	return c.Count()
}

func (p parser) MinMaxSumCount(mmsc aggregation.MinMaxSumCount) (interface{}, interface{}, interface{}, interface{}, error) {
	min, err := p.Min(mmsc)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	max, err := p.Max(mmsc)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	sum, err := p.Sum(mmsc)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	count, err := p.Count(mmsc)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return min, max, sum, count, nil
}

func (p parser) LastValue(l aggregation.LastValue) (interface{}, *time.Time, error) {
	v, ts, err := l.LastValue()
	if err != nil {
		return nil, nil, err
	}
	iv := v.AsInterface(p.Kind)
	if p.Config.DoNotPrintTime {
		return iv, nil, nil
	}
	return iv, &ts, nil
}

func (p parser) Quantile(q aggregation.Quantile) ([]quantile, error) {
	vals := make([]quantile, len(p.Config.Quantiles))
	for i, n := range p.Config.Quantiles {
		v, err := p.nToI(q.Quantile(n))
		if err != nil {
			return nil, err
		}
		vals[i] = quantile{Q: n, V: v}
	}
	return vals, nil
}

func (p parser) Distribution(d aggregation.Distribution) ([]quantile, interface{}, interface{}, interface{}, interface{}, error) {
	min, max, sum, count, err := p.MinMaxSumCount(d)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	dist, err := p.Quantile(d)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	return dist, min, max, sum, count, nil
}

func (p parser) Histogram(h aggregation.Histogram) (interface{}, []bucket, error) {
	sum, err := p.Sum(h)
	if err != nil {
		return nil, nil, err
	}

	bkts, err := h.Histogram()
	if err != nil {
		return nil, nil, err
	}
	if len(bkts.Boundaries) != len(bkts.Counts)+1 {
		return nil, nil, errors.New("histogram boundaries do not match counts")
	}
	hist := make([]bucket, 0, len(bkts.Boundaries)+1)
	nextMin := math.Inf(-1)
	for i, bound := range bkts.Boundaries {
		b := bucket{}
		b.Min, nextMin = nextMin, bound
		b.Max = bound
		if i == 0 {
			b.Min = math.Inf(-1)
		}
		b.Count = bkts.Counts[i]
		hist = append(hist, b)
	}
	hist = append(hist, bucket{
		boundary: boundary{
			Min: nextMin,
			Max: math.Inf(1),
		},
		Count: bkts.Boundaries[len(bkts.Counts)-1],
	})

	return sum, hist, nil
}

func (p parser) Points(points aggregation.Points) ([]point, error) {
	raws, err := points.Points()
	if err != nil {
		return nil, err
	}
	pts := make([]point, 0, len(raws))
	for _, raw := range raws {
		pts = append(pts, point{
			Value: raw.AsInterface(p.Kind),
		})
	}
	return pts, nil
}
