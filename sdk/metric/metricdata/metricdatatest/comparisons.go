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

package metricdatatest // import "go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/davecgh/go-spew/spew"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

var spewConfig = spew.ConfigState{
	Indent:                  " ",
	DisablePointerAddresses: true,
	DisableCapacities:       true,
	SortKeys:                true,
	MaxDepth:                10,
}

type failure interface {
	SetIndent(string)
	String() string
}

type equalityFailure struct {
	name     string
	indent   string
	esc      string
	expected any
	actual   any
}

func (f *equalityFailure) SetIndent(indent string) {
	f.indent = indent
}

func (f *equalityFailure) String() string {
	format := fmt.Sprintf(
		"%[1]s%%s not equal:\n%[1]s%[1]sexpected: %[2]s\n%[1]s%[1]sactual: %[2]s",
		f.indent, f.esc,
	)
	return fmt.Sprintf(format, f.name, f.expected, f.actual)
}

type diffFailure struct {
	name    string
	indent  string
	missing string
	extra   string
}

func (f *diffFailure) SetIndent(indent string) {
	f.indent = indent
}

func (f *diffFailure) String() string {
	var msg bytes.Buffer
	msg.WriteString(f.indent)
	msg.WriteString(f.name)
	msg.WriteString(" do not contain the same values:")
	if len(f.missing) > 0 {
		msg.WriteString("\n")
		msg.WriteString(f.indent)
		msg.WriteString(f.indent)
		msg.WriteString("missing expected values: ")
		msg.WriteString(f.missing)
	}
	if len(f.extra) > 0 {
		msg.WriteString("\n")
		msg.WriteString(f.indent)
		msg.WriteString(f.indent)
		msg.WriteString("unexpected additional values: ")
		msg.WriteString(f.extra)
	}
	return msg.String()
}

type strFailure struct {
	name   string
	indent string
	str    string
}

func (f *strFailure) SetIndent(indent string) {
	f.indent = indent
}

func (f *strFailure) String() string {
	return fmt.Sprintf("%s%s: %s", f.indent, f.name, f.str)
}

type comparison struct {
	object string

	failures []failure
}

func newComparison(object string) comparison {
	return comparison{object: object}
}

func (c *comparison) String() string {
	if len(c.failures) == 0 {
		return ""
	}

	var msg bytes.Buffer
	msg.WriteString(c.object)
	msg.WriteString(":\n")
	for _, f := range c.failures {
		msg.WriteString(f.String())
	}
	return msg.String()
}

func (c *comparison) AddComparison(other comparison) {
	if !other.Failed() {
		return
	}

	for _, f := range other.failures {
		f.SetIndent("\t\t")
		c.failures = append(c.failures, &strFailure{
			name:   other.object,
			indent: "\t",
			str:    f.String(),
		})
	}
}

func (c *comparison) AddFailure(f failure) {
	if f != nil {
		f.SetIndent("\t")
		c.failures = append(c.failures, f)
	}
}

func (c *comparison) NotEqual(name string, esc string, expected, actual any) {
	c.failures = append(c.failures, &equalityFailure{
		name:     name,
		indent:   "\t",
		esc:      esc,
		expected: expected,
		actual:   actual,
	})
}

func (c *comparison) NotEqualV(name string, expected, actual any) {
	c.NotEqual(name, "%v", expected, actual)
}

func (c *comparison) Failed() bool {
	return len(c.failures) > 0
}

// equalResourceMetrics returns reasons ResourceMetrics are not equal. If they
// are equal, the returned reasons will be empty.
//
// The ScopeMetrics each ResourceMetrics contains are compared based on
// containing the same ScopeMetrics, not the order they are stored in.
func equalResourceMetrics(a, b metricdata.ResourceMetrics) comparison {
	c := newComparison("ResourceMetrics")
	if !a.Resource.Equal(b.Resource) {
		c.NotEqualV("Resources", a.Resource, b.Resource)
	}

	c.AddFailure(fmtDiff(diffSlices(
		a.ScopeMetrics,
		b.ScopeMetrics,
		func(a, b metricdata.ScopeMetrics) bool {
			c := equalScopeMetrics(a, b)
			return !c.Failed()
		},
	))("ScopeMetrics"))
	return c
}

// equalScopeMetrics returns reasons ScopeMetrics are not equal. If they are
// equal, the returned reasons will be empty.
//
// The Metrics each ScopeMetrics contains are compared based on containing the
// same Metrics, not the order they are stored in.
func equalScopeMetrics(a, b metricdata.ScopeMetrics) comparison {
	c := newComparison("ScopeMetrics")
	if a.Scope != b.Scope {
		reasons = append(reasons, notEqStrV("Scope", a.Scope, b.Scope))
	}

	r := fmtDiff(diffSlices(
		a.Metrics,
		b.Metrics,
		func(a, b metricdata.Metrics) bool {
			r := equalMetrics(a, b)
			return len(r) == 0
		},
	))
	if r != "" {
		reasons = append(reasons, fmt.Sprintf("ScopeMetrics Metrics not equal:\n%s", r))
	}
	return c
}

// equalMetrics returns reasons Metrics are not equal. If they are equal, the
// returned reasons will be empty.
func equalMetrics(a, b metricdata.Metrics) comparison {
	c := newComparison("Metrics")
	if a.Name != b.Name {
		reasons = append(reasons, notEqStrV("Name", a.Name, b.Name))
	}
	if a.Description != b.Description {
		reasons = append(reasons, notEqStrV("Description", a.Description, b.Description))
	}
	if a.Unit != b.Unit {
		reasons = append(reasons, notEqStrV("Unit", a.Unit, b.Unit))
	}

	r := equalAggregations(a.Data, b.Data)
	if len(r) > 0 {
		reasons = append(reasons, "Metrics Data not equal:")
		reasons = append(reasons, r...)
	}
	return c
}

// equalAggregations returns reasons a and b are not equal. If they are equal,
// the returned reasons will be empty.
func equalAggregations(a, b metricdata.Aggregation) comparison {
	c := newComparison("Aggregation")
	if a == nil || b == nil {
		if a != b {
			return []string{notEqStrV("Aggregation", a, b)}
		}
		return reasons
	}

	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return []string{fmt.Sprintf("Aggregation types not equal:\nexpected: %T\nactual: %T", a, b)}
	}

	switch v := a.(type) {
	case metricdata.Gauge:
		r := equalGauges(v, b.(metricdata.Gauge))
		if len(r) > 0 {
			r[0] = fmt.Sprintf("Gauge: %s", r[0])
			reasons = append(reasons, r...)
		}
	case metricdata.Sum:
		r := equalSums(v, b.(metricdata.Sum))
		if len(r) > 0 {
			r[0] = fmt.Sprintf("Sum: %s", r[0])
			reasons = append(reasons, r...)
		}
	case metricdata.Histogram:
		r := equalHistograms(v, b.(metricdata.Histogram))
		if len(r) > 0 {
			r[0] = fmt.Sprintf("Histogram: %s", r[0])
			reasons = append(reasons, r...)
		}
	default:
		reasons = append(reasons, fmt.Sprintf("Aggregation of unknown types %T", a))
	}
	return c
}

// equalGauges returns reasons Gauges are not equal. If they are equal, the
// returned reasons will be empty.
//
// The DataPoints each Gauge contains are compared based on containing the
// same DataPoints, not the order they are stored in.
func equalGauges(a, b metricdata.Gauge) comparison {
	c := newComparison("Gauge")
	r := fmtDiff(diffSlices(
		a.DataPoints,
		b.DataPoints,
		func(a, b metricdata.DataPoint) bool {
			r := equalDataPoints(a, b)
			return len(r) == 0
		},
	))
	if r != "" {
		reasons = append(reasons, fmt.Sprintf("Gauge DataPoints not equal:\n%s", r))
	}
	return c
}

// equalSums returns reasons Sums are not equal. If they are equal, the
// returned reasons will be empty.
//
// The DataPoints each Sum contains are compared based on containing the same
// DataPoints, not the order they are stored in.
func equalSums(a, b metricdata.Sum) comparison {
	c := newComparison("Sum")
	if a.Temporality != b.Temporality {
		reasons = append(reasons, notEqStrV("Temporality", a.Temporality, b.Temporality))
	}
	if a.IsMonotonic != b.IsMonotonic {
		reasons = append(reasons, notEqStrV("IsMonotonic", a.IsMonotonic, b.IsMonotonic))
	}

	r := fmtDiff(diffSlices(
		a.DataPoints,
		b.DataPoints,
		func(a, b metricdata.DataPoint) bool {
			r := equalDataPoints(a, b)
			return len(r) == 0
		},
	))
	if r != "" {
		reasons = append(reasons, fmt.Sprintf("Sum DataPoints not equal:\n%s", r))
	}
	return reasons
}

// equalHistograms returns reasons Histograms are not equal. If they are
// equal, the returned reasons will be empty.
//
// The DataPoints each Histogram contains are compared based on containing the
// same HistogramDataPoint, not the order they are stored in.
func equalHistograms(a, b metricdata.Histogram) comparison {
	c := newComparison("Histogram")
	if a.Temporality != b.Temporality {
		c.NotEqualV("Temporality", a.Temporality, b.Temporality)
	}

	c.AddFailure(fmtDiff(diffSlices(
		a.DataPoints,
		b.DataPoints,
		func(a, b metricdata.HistogramDataPoint) bool {
			c := equalHistogramDataPoints(a, b)
			return !c.Failed()
		},
	))("HistogramDataPoints"))
	return c
}

// equalDataPoints returns reasons DataPoints are not equal. If they are
// equal, the returned reasons will be empty.
func equalDataPoints(a, b metricdata.DataPoint) comparison {
	c := newComparison("DataPoint")
	if !a.Attributes.Equals(&b.Attributes) {
		c.NotEqual(
			"Attributes", "%s",
			a.Attributes.Encoded(attribute.DefaultEncoder()),
			b.Attributes.Encoded(attribute.DefaultEncoder()),
		)
	}
	if !a.StartTime.Equal(b.StartTime) {
		c.NotEqualV("StartTime", a.StartTime.UnixNano(), b.StartTime.UnixNano())
	}
	if !a.Time.Equal(b.Time) {
		c.NotEqualV("Time", a.Time.UnixNano(), b.Time.UnixNano())
	}

	c.AddComparison(equalValues(a.Value, b.Value))
	return c
}

// equalHistogramDataPoints returns reasons HistogramDataPoints are not equal.
// If they are equal, the returned reasons will be empty.
func equalHistogramDataPoints(a, b metricdata.HistogramDataPoint) comparison {
	c := newComparison("HistogramDataPoint")
	if !a.Attributes.Equals(&b.Attributes) {
		c.NotEqual(
			"Attributes", "%s",
			a.Attributes.Encoded(attribute.DefaultEncoder()),
			b.Attributes.Encoded(attribute.DefaultEncoder()),
		)
	}
	if !a.StartTime.Equal(b.StartTime) {
		c.NotEqualV("StartTime", a.StartTime.UnixNano(), b.StartTime.UnixNano())
	}
	if !a.Time.Equal(b.Time) {
		c.NotEqualV("Time", a.Time.UnixNano(), b.Time.UnixNano())
	}
	if a.Count != b.Count {
		c.NotEqualV("Count", a.Count, b.Count)
	}
	if !equalSlices(a.Bounds, b.Bounds) {
		c.NotEqualV("Bounds", a.Bounds, b.Bounds)
	}
	if !equalSlices(a.BucketCounts, b.BucketCounts) {
		c.NotEqualV("BucketCounts", a.BucketCounts, b.BucketCounts)
	}
	if !equalPtrValues(a.Min, b.Min) {
		c.NotEqualV("Min", a.Min, b.Min)
	}
	if !equalPtrValues(a.Max, b.Max) {
		c.NotEqualV("Max", a.Max, b.Max)
	}
	if a.Sum != b.Sum {
		c.NotEqualV("Sum", a.Sum, b.Sum)
	}
	return c
}

// equalValues returns reasons Values are not equal. If they are equal, the
// returned reasons will be empty.
func equalValues(a, b metricdata.Value) comparison {
	c := newComparison("Value")
	if a == nil || b == nil {
		if a != b {
			c.NotEqualV("values", a, b)
		}
		return c
	}

	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		c.NotEqual("types", "%T", a, b)
		return c
	}

	switch v := a.(type) {
	case metricdata.Int64:
		c.AddComparison(equalInt64(v, b.(metricdata.Int64)))
	case metricdata.Float64:
		c.AddComparison(equalFloat64(v, b.(metricdata.Float64)))
	default:
		c.AddFailure(&strFailure{
			name: "type",
			str:  fmt.Sprintf("unknown type: %T", a),
		})
	}

	return c
}

// equalFloat64 returns reasons Float64s are not equal. If they are equal, the
// returned reasons will be empty.
func equalFloat64(a, b metricdata.Float64) comparison {
	c := newComparison("Float64")
	if a != b {
		c.NotEqualV("value", a, b)
	}
	return c
}

// equalInt64 returns reasons Int64s are not equal. If they are equal, the
// returned reasons will be empty.
func equalInt64(a, b metricdata.Int64) comparison {
	c := newComparison("Int64")
	if a != b {
		c.NotEqualV("value", a, b)
	}
	return c
}

func equalSlices[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func equalPtrValues[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

func diffSlices[T any](a, fmtDiff []T, equal func(T, T) bool) (extraA, extraB []T) {
	visited := make([]bool, len(fmtDiff))
	for i := 0; i < len(a); i++ {
		found := false
		for j := 0; j < len(fmtDiff); j++ {
			if visited[j] {
				continue
			}
			if equal(a[i], fmtDiff[j]) {
				visited[j] = true
				found = true
				break
			}
		}
		if !found {
			extraA = append(extraA, a[i])
		}
	}

	for j := 0; j < len(fmtDiff); j++ {
		if visited[j] {
			continue
		}
		extraB = append(extraB, fmtDiff[j])
	}

	return extraA, extraB
}

func fmtDiff[T any](extraExpected, extraActual []T) func(string) failure {
	return func(name string) failure {
		var missing, extra string
		if len(extraExpected) > 0 {
			missing = spewConfig.Sdump(extraExpected)
		}
		if len(extraActual) > 0 {
			extra = spewConfig.Sdump(extraActual)
		}

		if missing == "" && extra == "" {
			return nil
		}
		return &diffFailure{
			name:    name,
			missing: missing,
			extra:   extra,
		}
	}
}
