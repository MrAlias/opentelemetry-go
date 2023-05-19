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
	"runtime"
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
	Store(KeyValue) builder
	Build() (builder, Set)
}

var sliceBuilderPool = sync.Pool{
	New: func() any { return new(sliceBuilder) },
}

func getSliceBuilder() *sliceBuilder {
	b := sliceBuilderPool.Get().(*sliceBuilder)
	runtime.SetFinalizer(b, func(sb *sliceBuilder) {
		// If the Builder containing the *sliceBuilder becomes unreachable,
		// return it to the pool instead of garbage collecting.
		sliceBuilderPool.Put(sb)
	})
	return b
}

func putSliceBuilder(b *sliceBuilder) {
	// Clear the finalize so it doesn't interfere with the Pool.
	runtime.SetFinalizer(b, nil)
	sliceBuilderPool.Put(b)
}

type sliceBuilder struct {
	data []KeyValue
}

func newSliceBuilder(data []KeyValue) *sliceBuilder {
	b := getSliceBuilder()
	b.setLenAndCap(len(data), len(data))
	copy(b.data, data)

	if b.len() <= 1 {
		return b
	}

	srt := sortables.Get().(*Sortable)
	*srt = b.data
	sort.Stable(srt)
	*srt = nil
	sortables.Put(srt)

	// De-duplicate with last-value-wins.
	cursor := b.len() - 1
	for i := b.len() - 2; i >= 0; i-- {
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

func (b *sliceBuilder) len() int {
	return len(b.data)
}

func (b *sliceBuilder) Store(kv KeyValue) builder {
	idx := sort.Search(b.len(), func(i int) bool {
		return b.data[i].Key >= kv.Key
	})

	if exist, ok := b.get(idx); ok && exist.Key == kv.Key {
		b.replace(idx, kv)
	} else {
		b.insert(idx, kv)
	}
	return b
}

// get returns the KeyValue at index with true if it exists. Otherwise, if
// index is out of the range of data, it returns an empty key and false.
func (b *sliceBuilder) get(index int) (KeyValue, bool) {
	if index < 0 || index >= b.len() {
		return KeyValue{}, false
	}
	return b.data[index], true
}

// insert inserts kv at index, expanding the data by one.
func (b *sliceBuilder) insert(index int, kv KeyValue) {
	if index >= b.len() {
		b.data = append(b.data, kv)
		return
	}
	b.data = append(b.data[:index+1], b.data[index:]...)
	b.data[index] = kv
}

// replace replaces the value at index with kv.
func (b *sliceBuilder) replace(index int, kv KeyValue) {
	b.data[index] = kv
}

func (b *sliceBuilder) Build() (builder, Set) {
	defer putSliceBuilder(b)
	ab := newArrayBuilder(b.data)
	return ab.Build()
}

var arrayBuilderPool = sync.Pool{
	New: func() any { return new(arrayBuilder) },
}

func getArrayBuilder() *arrayBuilder {
	b := arrayBuilderPool.Get().(*arrayBuilder)
	runtime.SetFinalizer(b, func(sb *arrayBuilder) {
		// If the Builder containing the *arrayBuilder becomes unreachable,
		// return it to the pool instead of garbage collecting.
		arrayBuilderPool.Put(sb)
	})
	return b
}

func putArrayBuilder(b *arrayBuilder) {
	// Clear the finalize so it doesn't interfere with the Pool.
	runtime.SetFinalizer(b, nil)
	arrayBuilderPool.Put(b)
}

type arrayBuilder struct {
	// TODO: *[..]KeyValue (i.e. reflect value of a pointer to a fixed size array of
	// KeyValues).
	data reflect.Value
	// immutable is true if data is shared with a returned Set.
	immutable bool
}

func newArrayBuilder(data []KeyValue) *arrayBuilder {
	b := getArrayBuilder()
	/*
		if b.len() != len(data) {
			// Common case.
			b.data = reflect.New(reflect.ArrayOf(len(data), keyValueType))
		}
		reflect.Copy(b.data.Elem(), reflect.ValueOf(data))
	*/
	b.data = reflect.ValueOf(computeIface(data))
	return b
}

func (b *arrayBuilder) len() int {
	return b.data.Len()
}

func (b *arrayBuilder) Store(kv KeyValue) builder {
	n := b.len()
	idx := sort.Search(n, func(i int) bool {
		return b.data.Index(i).Interface().(KeyValue).Key >= kv.Key
	})

	if idx >= n {
		defer putArrayBuilder(b)

		sb := b.toSliceBuilder(n + 1)
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

		defer putArrayBuilder(b)

		sb := b.toSliceBuilder(n)
		sb.replace(idx, kv)
		return sb
	}

	defer putArrayBuilder(b)

	sb := b.toSliceBuilder(n + 1)
	sb.insert(idx, kv)
	return sb
}

func (b *arrayBuilder) toSliceBuilder(capacity int) *sliceBuilder {
	if capacity < b.len() {
		capacity = b.len()
	}
	sb := getSliceBuilder()
	sb.setLenAndCap(b.len(), capacity)
	reflect.Copy(reflect.ValueOf(sb.data), b.data)
	return sb
}

func (b *arrayBuilder) Build() (builder, Set) {
	b.immutable = true
	return b, Set{equivalent: Distinct{iface: b.data.Interface()}}
}
