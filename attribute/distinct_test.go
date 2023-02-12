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
	"testing"

	"github.com/stretchr/testify/assert"
)

var tests = []struct {
	name       string
	attr       Set
	attrPrime  Set
	attrAltKey Set
	attrAltVal Set
}{
	{
		name:       "Bool",
		attr:       NewSet(Bool("k", true)),
		attrPrime:  NewSet(Bool("k", true)),
		attrAltKey: NewSet(Bool("j", true)),
		attrAltVal: NewSet(Bool("k", false)),
	},
	{
		name:       "BoolSlice",
		attr:       NewSet(BoolSlice("k", []bool{true, false, true})),
		attrPrime:  NewSet(BoolSlice("k", []bool{true, false, true})),
		attrAltKey: NewSet(BoolSlice("j", []bool{true, false, true})),
		attrAltVal: NewSet(BoolSlice("k", []bool{false, true, true})),
	},
	{
		name:       "Int",
		attr:       NewSet(Int("k", 1)),
		attrPrime:  NewSet(Int("k", 1)),
		attrAltKey: NewSet(Int("j", 1)),
		attrAltVal: NewSet(Int("k", -1)),
	},
	{
		name:       "IntSlice",
		attr:       NewSet(IntSlice("k", []int{-3, 0, 1})),
		attrPrime:  NewSet(IntSlice("k", []int{-3, 0, 1})),
		attrAltKey: NewSet(IntSlice("j", []int{-3, 0, 1})),
		attrAltVal: NewSet(IntSlice("k", []int{-4, 0, 2})),
	},
	{
		name:       "Int64",
		attr:       NewSet(Int64("k", -1)),
		attrPrime:  NewSet(Int64("k", -1)),
		attrAltKey: NewSet(Int64("j", -1)),
		attrAltVal: NewSet(Int64("k", 1)),
	},
	{
		name:       "Int64Slice",
		attr:       NewSet(Int64Slice("k", []int64{-4, 0, 2})),
		attrPrime:  NewSet(Int64Slice("k", []int64{-4, 0, 2})),
		attrAltKey: NewSet(Int64Slice("j", []int64{-4, 0, 2})),
		attrAltVal: NewSet(Int64Slice("k", []int64{-3, 0, 1})),
	},
	{
		name:       "Float64",
		attr:       NewSet(Float64("k", 2.3)),
		attrPrime:  NewSet(Float64("k", 2.3)),
		attrAltKey: NewSet(Float64("j", 2.3)),
		attrAltVal: NewSet(Float64("k", -1e3)),
	},
	{
		name:       "Float64Slice",
		attr:       NewSet(Float64Slice("k", []float64{-1.2, 0.32, 1e9})),
		attrPrime:  NewSet(Float64Slice("k", []float64{-1.2, 0.32, 1e9})),
		attrAltKey: NewSet(Float64Slice("j", []float64{-1.2, 0.32, 1e9})),
		attrAltVal: NewSet(Float64Slice("k", []float64{-1.2, 1e9, 0.32})),
	},
	{
		name:       "String",
		attr:       NewSet(String("k", "val")),
		attrPrime:  NewSet(String("k", "val")),
		attrAltKey: NewSet(String("j", "val")),
		attrAltVal: NewSet(String("k", "alt")),
	},
	{
		name:       "StringSlice",
		attr:       NewSet(StringSlice("k", []string{"zero", "one", ""})),
		attrPrime:  NewSet(StringSlice("k", []string{"zero", "one", ""})),
		attrAltKey: NewSet(StringSlice("j", []string{"zero", "one", ""})),
		attrAltVal: NewSet(StringSlice("k", []string{"", "one", "zero"})),
	},
	{
		name: "All",
		attr: NewSet(
			Bool("k", true),
			BoolSlice("k", []bool{true, false, true}),
			Int("k", 1),
			IntSlice("k", []int{-3, 0, 1}),
			Int64("k", -1),
			Int64Slice("k", []int64{-4, 0, 2}),
			Float64("k", 2.3),
			Float64Slice("k", []float64{-1.2, 0.32, 1e9}),
			String("k", "val"),
			StringSlice("k", []string{"zero", "one", ""}),
		),
		attrPrime: NewSet(
			Bool("k", true),
			BoolSlice("k", []bool{true, false, true}),
			Int("k", 1),
			IntSlice("k", []int{-3, 0, 1}),
			Int64("k", -1),
			Int64Slice("k", []int64{-4, 0, 2}),
			Float64("k", 2.3),
			Float64Slice("k", []float64{-1.2, 0.32, 1e9}),
			String("k", "val"),
			StringSlice("k", []string{"zero", "one", ""}),
		),
		attrAltKey: NewSet(
			Bool("j", true),
			BoolSlice("j", []bool{true, false, true}),
			Int("j", 1),
			IntSlice("j", []int{-3, 0, 1}),
			Int64("j", -1),
			Int64Slice("j", []int64{-4, 0, 2}),
			Float64("j", 2.3),
			Float64Slice("j", []float64{-1.2, 0.32, 1e9}),
			String("j", "val"),
			StringSlice("j", []string{"zero", "one", ""}),
		),
		attrAltVal: NewSet(
			Bool("k", false),
			BoolSlice("k", []bool{false, true, true}),
			Int("k", -1),
			IntSlice("k", []int{-4, 0, 2}),
			Int64("k", 1),
			Int64Slice("k", []int64{-3, 0, 1}),
			Float64("k", -1e3),
			Float64Slice("k", []float64{-1.2, 1e9, 0.32}),
			String("k", "alt"),
			StringSlice("k", []string{"", "one", "zero"}),
		),
	},
}

func TestDistinct(t *testing.T) {
	for _, test := range tests {
		f := testDistinct(test.attr, test.attrPrime, test.attrAltKey, test.attrAltVal)
		t.Run(test.name, f)
	}
}

func testDistinct(a, ap, ak, av Set) func(*testing.T) {
	return func(t *testing.T) {
		d := a.Equivalent()
		assert.Equal(t, d, a.Equivalent(), "same keys/values")
		assert.Equal(t, d, ap.Equivalent(), "equivalent keys/values")
		assert.NotEqual(t, d, ak.Equivalent(), "different keys")
		assert.NotEqual(t, d, av.Equivalent(), "different values")
	}
}

var benchmarks = []struct {
	name string
	attr Set
}{
	{name: "Bool", attr: NewSet(Bool("k", true))},
	{name: "BoolSlice", attr: NewSet(BoolSlice("k", []bool{true, false, true}))},
	{name: "Int", attr: NewSet(Int("k", 1))},
	{name: "IntSlice", attr: NewSet(IntSlice("k", []int{-3, 0, 1}))},
	{name: "Int64", attr: NewSet(Int64("k", -1))},
	{name: "Int64Slice", attr: NewSet(Int64Slice("k", []int64{-4, 0, 2}))},
	{name: "Float64", attr: NewSet(Float64("k", 2.3))},
	{name: "Float64Slice", attr: NewSet(Float64Slice("k", []float64{-1.2, 0.32, 1e9}))},
	{name: "String", attr: NewSet(String("k", "val"))},
	{name: "StringSlice", attr: NewSet(StringSlice("k", []string{"zero", "one", ""}))},
	{
		name: "All",
		attr: NewSet(
			Bool("k", true),
			BoolSlice("k", []bool{true, false, true}),
			Int("k", 1),
			IntSlice("k", []int{-3, 0, 1}),
			Int64("k", -1),
			Int64Slice("k", []int64{-4, 0, 2}),
			Float64("k", 2.3),
			Float64Slice("k", []float64{-1.2, 0.32, 1e9}),
			String("k", "val"),
			StringSlice("k", []string{"zero", "one", ""}),
		),
	},
}

func BenchmarkHash(b *testing.B) {
	for _, bench := range benchmarks {
		b.Run(bench.name, benchHash(bench.attr.Iter()))
	}
}

var hashResult uint64

func benchHash(iter Iterator) func(*testing.B) {
	return func(b *testing.B) {
		b.ReportAllocs()
		for n := 0; n < b.N; n++ {
			hashResult = hash(iter)
		}
	}
}
