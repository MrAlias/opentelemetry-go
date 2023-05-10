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
	"fmt"
	"reflect"

	"go.opentelemetry.io/otel/attribute/internal/fnv"
)

// Distinct is a pseudo-unique and comparable representation of KeyValues. This
// value is suitable to be used as a map key.
type Distinct struct {
	value fnv.Hash
}

// NewDistinct returns a Distinct computed for the passed kvs.
//
// The returned Distinct is generated using a non-cryptographic hash function
// and should not be used for any security sensitive operations.
func NewDistinct(kvs []KeyValue) Distinct {
	h := fnv.New()
	for _, kv := range kvs {
		h = h.String(string(kv.Key))

		switch kv.Value.Type() {
		case BOOL:
			h = h.Bool(kv.Value.AsBool())
		case INT64:
			h = h.Int64(kv.Value.AsInt64())
		case FLOAT64:
			h = h.Float64(kv.Value.AsFloat64())
		case STRING:
			h = h.String(kv.Value.AsString())
		case BOOLSLICE:
			// Avoid allocating a new []bool with AsBoolSlice.
			rv := reflect.ValueOf(kv.Value.slice)
			for i := 0; i < rv.Len(); i++ {
				h = h.Bool(rv.Index(i).Bool())
			}
		case INT64SLICE:
			// Avoid allocating a new []int64 with AsInt64Slice.
			rv := reflect.ValueOf(kv.Value.slice)
			for i := 0; i < rv.Len(); i++ {
				h = h.Int64(rv.Index(i).Int())
			}
		case FLOAT64SLICE:
			// Avoid allocating a new []float64 with AsFloat64Slice.
			rv := reflect.ValueOf(kv.Value.slice)
			for i := 0; i < rv.Len(); i++ {
				h = h.Float64(rv.Index(i).Float())
			}
		case STRINGSLICE:
			// Avoid allocating a new []string with AsStringSlice.
			rv := reflect.ValueOf(kv.Value.slice)
			for i := 0; i < rv.Len(); i++ {
				h = h.String(rv.Index(i).String())
			}
		default:
			// Logging is an alternative, but using the internal logger here
			// causes an import cycle so it is not done.
			v := kv.Value.AsInterface()
			msg := fmt.Sprintf("unknown value type: %[1]v (%[1]T)", v)
			panic(msg)
		}
	}
	return Distinct{value: h}
}

// Valid returns true if d is a non-empty Distinct value.
func (d Distinct) Valid() bool {
	// Given the low probability of collision for FNV-1a, this is a good guess.
	return d.value != 0
}
