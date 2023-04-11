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

package instrument // import "go.opentelemetry.io/otel/metric/instrument"

import "go.opentelemetry.io/otel/attribute"

// AddOption[N] applies options to an addition measurement. See
// [MeasurementOption] for other options that can be used as a AddOption[N].
type AddOption[N int64 | float64] interface {
	applyAdd(AddConfig[N]) AddConfig[N]
}

// AddConfig[N] contains options for an addition measurement.
type AddConfig[N int64 | float64] struct {
	attrs attribute.Set
}

// NewAddConfig[N] returns a new [AddConfig[N]] with all opts applied.
func NewAddConfig[N int64 | float64](opts []AddOption[N]) AddConfig[N] {
	config := AddConfig[N]{attrs: *attribute.EmptySet()}
	for _, o := range opts {
		config = o.applyAdd(config)
	}
	return config
}

// Attributes returns the configured attribute set.
func (c AddConfig[N]) Attributes() attribute.Set {
	return c.attrs
}

// RecordOption[N] applies options to an addition measurement. See
// [MeasurementOption] for other options that can be used as a
// RecordOption.
type RecordOption[N int64 | float64] interface {
	applyRecord(RecordConfig[N]) RecordConfig[N]
}

// RecordConfig[N] contains options for an int64 recorded measurement.
type RecordConfig[N int64 | float64] struct {
	attrs attribute.Set
}

// NewRecordConfig[N] returns a new [RecordConfig[N]] with all opts
// applied.
func NewRecordConfig[N int64 | float64](opts []RecordOption[N]) RecordConfig[N] {
	config := RecordConfig[N]{attrs: *attribute.EmptySet()}
	for _, o := range opts {
		config = o.applyRecord(config)
	}
	return config
}

// Attributes returns the configured attribute set.
func (c RecordConfig[N]) Attributes() attribute.Set {
	return c.attrs
}

// ObserveOption[N] applies options to an addition measurement. See
// [MeasurementOption] for other options that can be used as a
// ObserveOption[N].
type ObserveOption[N int64 | float64] interface {
	applyObserve(ObserveConfig[N]) ObserveConfig[N]
}

// ObserveConfig[N] contains options for an int64 observed measurement.
type ObserveConfig[N int64 | float64] struct {
	attrs attribute.Set
}

// NewObserveConfig[N] returns a new [ObserveConfig[N]] with all opts
// applied.
func NewObserveConfig[N int64 | float64](opts []ObserveOption[N]) ObserveConfig[N] {
	config := ObserveConfig[N]{attrs: *attribute.EmptySet()}
	for _, o := range opts {
		config = o.applyObserve(config)
	}
	return config
}

// Attributes returns the configured attribute set.
func (c ObserveConfig[N]) Attributes() attribute.Set {
	return c.attrs
}

// MeasurementOption[N] applies options to all instrument measurement.
type MeasurementOption[N int64 | float64] interface {
	AddOption[N]
	RecordOption[N]
	ObserveOption[N]
}

type attrOpt[N int64 | float64] struct {
	set attribute.Set
}

// mergeSets returns the union of keys between a and b. Any duplicate keys will
// use the value associated with b.
func mergeSets(a, b attribute.Set) attribute.Set {
	// NewMergeIterator uses the first value for any duplicates.
	iter := attribute.NewMergeIterator(&b, &a)
	merged := make([]attribute.KeyValue, 0, a.Len()+b.Len())
	for iter.Next() {
		merged = append(merged, iter.Attribute())
	}
	return attribute.NewSet(merged...)
}

func (o attrOpt[N]) applyAdd(c AddConfig[N]) AddConfig[N] {
	switch {
	case o.set.Len() == 0:
	case c.attrs.Len() == 0:
		c.attrs = o.set
	default:
		c.attrs = mergeSets(c.attrs, o.set)
	}
	return c
}

func (o attrOpt[N]) applyRecord(c RecordConfig[N]) RecordConfig[N] {
	switch {
	case o.set.Len() == 0:
	case c.attrs.Len() == 0:
		c.attrs = o.set
	default:
		c.attrs = mergeSets(c.attrs, o.set)
	}
	return c
}
func (o attrOpt[N]) applyObserve(c ObserveConfig[N]) ObserveConfig[N] {
	switch {
	case o.set.Len() == 0:
	case c.attrs.Len() == 0:
		c.attrs = o.set
	default:
		c.attrs = mergeSets(c.attrs, o.set)
	}
	return c
}

// WithAttributeSet sets the attribute Set associated with a measurement is
// made with.
//
// If multiple WithAttributeSet or WithAttributes options are passed the
// attributes will be merged together in the order they are passed. Attributes
// with duplicate keys will use the last value passed.
func WithAttributeSet[N int64 | float64](attributes attribute.Set) MeasurementOption[N] {
	return attrOpt[N]{set: attributes}
}

// WithAttribute converts attributes into an attribute Set and sets the Set
// to be associated with a measurement. This is shorthand for:
//
//	cp := make([]attribute.KeyValue, len(attributes))
//	copy(cp, attributes)
//	WithAttributes(attribute.NewSet(cp...))
//
// [attribute.NewSet] may modify the passed attributes so this will make a copy of
// attributes before creating a set in order to ensure this function is
// concurrent safe. This makes this option function less optimized in
// comparison to [WithAttributeSet]. That option function should be perfered
// for performance sensitive code.
//
// See [WithAttributeSet] for information about how multiple WithAttributes are
// merged.
func WithAttributes[N int64 | float64](attributes ...attribute.KeyValue) MeasurementOption[N] {
	cp := make([]attribute.KeyValue, len(attributes))
	copy(cp, attributes)
	return attrOpt[N]{set: attribute.NewSet(cp...)}
}
