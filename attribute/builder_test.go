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
	"strconv"
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

func BenchmarkSingleAttrChangeSet(b *testing.B) {
	b.Run("1", benchmarkSingleAttrChangeSet(1))
	b.Run("10", benchmarkSingleAttrChangeSet(10))
	b.Run("100", benchmarkSingleAttrChangeSet(100))
}

func benchmarkSingleAttrChangeSet(n int) func(*testing.B) {
	base := make([]KeyValue, n)
	for i := range base[:n-1] {
		base[i] = Int("base"+strconv.Itoa(i), i)
	}
	base[len(base)-1] = Int("incr", -1)
	return func(b *testing.B) {
		b.Run("NewSet", func(b *testing.B) {
			attr := snapshot(base)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				attr[len(attr)-1] = Int("incr", i)
				s := NewSet(attr...)
				if s.Len() != n {
					b.Errorf("set length not %d: %d", n, s.Len())
				}
			}
		})
		b.Run("Builder", func(b *testing.B) {
			attr := snapshot(base)
			build := NewBuilder(attr)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				build.Store(Int("incr", i))
				s := build.Build()
				if s.Len() != n {
					b.Errorf("set length not %d: %d", n, s.Len())
				}
			}
		})
	}
}
