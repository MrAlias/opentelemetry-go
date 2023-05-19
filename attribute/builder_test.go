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

package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func snapshot(in []KeyValue) []KeyValue {
	cp := make([]KeyValue, len(in))
	copy(cp, in)
	return cp
}

func TestBuilder(t *testing.T) {
	kvs := []KeyValue{
		Int("a", 0),
		Int("b", 0),
		Int("d", 0),
		Int("b", 1),
		Int("a", 1),
		Int("b", 2),
		Int("c", 0),
		Int("b", 3),
		Int("d", 1),
		Int("a", 2),
		Int("b", 4),
	}

	b := NewBuilder(kvs)
	want := []KeyValue{
		Int("a", 2),
		Int("b", 4),
		Int("c", 0),
		Int("d", 1),
	}

	s0 := b.Build()
	want0 := snapshot(want)
	assert.Equal(t, want0, s0.ToSlice())

	// Override
	want[0] = Int("a", 3)
	b.Store(want[0])

	s1 := b.Build()
	want1 := snapshot(want)
	assert.Equal(t, want1, s1.ToSlice(), "override")
	assert.Equal(t, want0, s0.ToSlice(), "set modified")

	// Insert
	want = append(want[:2], want[1:]...)
	want[1] = Int("aa", 1)
	b.Store(want[1])

	s2 := b.Build()
	want2 := snapshot(want)
	assert.Equal(t, want2, s2.ToSlice(), "insert")
	assert.Equal(t, want1, s1.ToSlice(), "set modified")
	assert.Equal(t, want0, s0.ToSlice(), "set modified")

	// Append
	want = append(want, Int("e", 0))
	b.Store(want[len(want)-1])

	s3 := b.Build()
	want3 := snapshot(want)
	assert.Equal(t, want3, s3.ToSlice(), "append")
	assert.Equal(t, want2, s2.ToSlice(), "set modified")
	assert.Equal(t, want1, s1.ToSlice(), "set modified")
	assert.Equal(t, want0, s0.ToSlice(), "set modified")
}
