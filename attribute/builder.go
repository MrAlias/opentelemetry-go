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
	"sort"
)

// Builder constructs a Set iteratively.
type Builder struct {
	data []KeyValue
}

func NewBuilder(kvs []KeyValue) *Builder {
	data := make([]KeyValue, len(kvs))
	copy(data, kvs)

	if len(data) <= 1 {
		return &Builder{data: data}
	}

	srt := sortables.Get().(*Sortable)
	*srt = data
	sort.Stable(srt)
	*srt = nil
	sortables.Put(srt)

	// De-duplicate with last-value-wins.
	cursor := len(data) - 1
	for i := len(data) - 2; i >= 0; i-- {
		if data[i].Key == data[cursor].Key {
			// Ensure the Value is garbage collected.
			data[i] = KeyValue{}
			continue
		}
		cursor--
		data[i], data[cursor] = data[cursor], data[i]
	}
	data = data[cursor:]

	return &Builder{data: data}
}

func (b *Builder) len() int {
	return len(b.data)
}

func (b *Builder) Store(kv KeyValue) {
	// TODO: update this to accept ...KeyValue
	idx := sort.Search(b.len(), func(i int) bool {
		return b.data[i].Key >= kv.Key
	})

	if exist, ok := b.get(idx); ok && exist.Key == kv.Key {
		b.replace(idx, kv)
	} else {
		b.insert(idx, kv)
	}
}

// get returns the KeyValue at index with true if it exists. Otherwise, if
// index is out of the range of data, it returns an empty key and false.
func (b *Builder) get(index int) (KeyValue, bool) {
	if index < 0 || index >= b.len() {
		return KeyValue{}, false
	}
	return b.data[index], true
}

// insert inserts kv at index, expanding the data by one.
func (b *Builder) insert(index int, kv KeyValue) {
	if index >= b.len() {
		b.data = append(b.data, kv)
		return
	}
	b.data = append(b.data[:index+1], b.data[index:]...)
	b.data[index] = kv
}

// replace replaces the value at index with kv.
func (b *Builder) replace(index int, kv KeyValue) {
	b.data[index] = kv
}

func (b *Builder) Build() Set {
	return Set{computeDistinct(b.data)}
}
