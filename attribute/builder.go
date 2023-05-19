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

package attribute // import "go.opentelemetry.io/otel/attribute"

import (
	"reflect"
	"sort"
	"sync"
)

// Builder constructs a Set iteratively.
type Builder struct {
	builder builder
}

func NewBuilder(data []KeyValue) *Builder {
	return &Builder{builder: newSliceBuilder(data)}
}

func (b *Builder) Len() int {
	return b.builder.Len()
}

func (b *Builder) Store(kv KeyValue) {
	// TODO: change this to accept ...KeyValue.
	b.builder = b.builder.Store(kv)
}

func (b *Builder) Build() Set {
	var s Set
	b.builder, s = b.builder.Build()
	return s
}

type builder interface {
	Len() int
	Store(KeyValue) builder
	Build() (builder, Set)
}

var sliceBuilderPool = sync.Pool{
	New: func() any { return new(sliceBuilder) },
}

type sliceBuilder struct {
	data []KeyValue
}

func newSliceBuilder(data []KeyValue) *sliceBuilder {
	b := sliceBuilderPool.Get().(*sliceBuilder)
	b.setLenAndCap(len(data), len(data))
	copy(b.data, data)

	if b.Len() <= 1 {
		return b
	}

	srt := sortables.Get().(*Sortable)
	*srt = b.data
	sort.Stable(srt)
	*srt = nil
	sortables.Put(srt)

	// De-duplicate with last-value-wins.
	cursor := b.Len() - 1
	for i := b.Len() - 2; i >= 0; i-- {
		if b.data[i].Key == b.data[cursor].Key {
			// Ensure the Value is garbage collected.
			b.data[i] = KeyValue{}
			continue
		}
		cursor--
		b.data[i], b.data[cursor] = b.data[cursor], b.data[i]
	}
	b.data = b.data[cursor:]

	return b
}

func (b *sliceBuilder) setLenAndCap(length, capacity int) {
	if cap(b.data) >= capacity {
		b.data = b.data[:length]
		return
	}
	b.data = make([]KeyValue, length, capacity)
}

func (b *sliceBuilder) Len() int {
	return len(b.data)
}

func (b *sliceBuilder) Store(kv KeyValue) builder {
	idx := sort.Search(b.Len(), func(i int) bool {
		return b.data[i].Key >= kv.Key
	})

	if idx >= b.Len() {
		b.data = append(b.data, kv)
		return b
	}

	if b.data[idx].Key == kv.Key {
		// Overwrite existing key.
		b.data[idx] = kv
		return b
	}

	// Insert new Key.
	b.data = append(b.data[:idx+1], b.data[idx:]...)
	b.data[idx] = kv
	return b
}

func (b *sliceBuilder) Build() (builder, Set) {
	defer sliceBuilderPool.Put(b)
	ab := newArrayBuilder(b.data)
	return ab.Build()
}

var arrayBuilderPool = sync.Pool{
	New: func() any { return new(arrayBuilder) },
}

type arrayBuilder struct {
	// TODO: *[..]KeyValue (i.e. reflect value of a pointer to a fixed size array of
	// KeyValues).
	data reflect.Value
	// immutable is true if data is shared with a returned Set.
	immutable bool
}

func newArrayBuilder(data []KeyValue) *arrayBuilder {
	b := arrayBuilderPool.Get().(*arrayBuilder)
	/*
		if b.Len() != len(data) {
			// Common case.
			b.data = reflect.New(reflect.ArrayOf(len(data), keyValueType))
		}
		reflect.Copy(b.data.Elem(), reflect.ValueOf(data))
	*/
	b.data = reflect.ValueOf(computeIface(data))
	return b
}

func (b *arrayBuilder) Len() int {
	return b.data.Len()
}

func (b *arrayBuilder) Store(kv KeyValue) builder {
	n := b.Len()
	idx := sort.Search(n, func(i int) bool {
		return b.data.Index(i).Interface().(KeyValue).Key >= kv.Key
	})

	if idx >= n {
		defer arrayBuilderPool.Put(b)

		sb := sliceBuilderPool.Get().(*sliceBuilder)
		sb.setLenAndCap(n, n+1)
		reflect.Copy(reflect.ValueOf(sb.data), b.data)
		sb.data = append(sb.data, kv)
		return sb
	}

	exist := b.data.Index(idx).Interface().(KeyValue)
	if exist.Key == kv.Key {
		// TODO: use a refelect value that is mutable but can be "snap-shotted"
		// so b.immutable can be removed. One possiblity is to use a
		// reflect.Value of *[...]KeyValue now that arrayBuilder is pooled.
		// This should mean that when you use Elem on it you still can provide
		// an immutable array to the Set, but we should be able to mutate the
		// pointed to value without it changing any previously returned values.
		if !b.immutable && b.data.Index(idx).CanSet() {
			b.data.Index(idx).Set(reflect.ValueOf(kv))
			return b
		}

		// If the above TODO is addressed, this can be removed.

		defer arrayBuilderPool.Put(b)

		sb := sliceBuilderPool.Get().(*sliceBuilder)
		sb.setLenAndCap(n, n)
		reflect.Copy(reflect.ValueOf(sb.data), b.data)
		sb.data[idx] = kv
		return sb
	}

	defer arrayBuilderPool.Put(b)

	sb := sliceBuilderPool.Get().(*sliceBuilder)
	sb.setLenAndCap(n, n+1)
	reflect.Copy(reflect.ValueOf(sb.data), b.data)
	sb.data = append(sb.data[:idx+1], sb.data[idx:]...)
	sb.data[idx] = kv
	return sb
}

func (b *arrayBuilder) Build() (builder, Set) {
	b.immutable = true
	return b, Set{equivalent: Distinct{iface: b.data.Interface()}}
}
