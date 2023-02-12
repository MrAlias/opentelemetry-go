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
	"hash/fnv"
	"math"
	"reflect"
)

// Distinct is a statistically unique representation of a set of attributes.
// This can be used as a map key or for equality checking between Sets.
type Distinct struct {
	valid bool
	hash  uint64
}

func newDistinct(iter Iterator) Distinct {
	return Distinct{
		valid: true,
		hash:  hash(iter),
	}
}

// Valid returns true if this value refers to a valid Set.
func (d Distinct) Valid() bool {
	return d.valid
}

func hash(iter Iterator) uint64 {
	h := fnv.New64a()
	for iter.Next() {
		a := iter.Attribute()

		_, _ = h.Write([]byte(a.Key))

		val := a.Value
		switch val.Type() {
		case BOOL:
			b := boolBytes(val.AsBool())
			_, _ = h.Write(b[:])
		case BOOLSLICE:
			rv := reflect.ValueOf(val.slice)
			for i := 0; i < rv.Len(); i++ {
				v := rv.Index(i)
				b := boolBytes(v.Bool())
				_, _ = h.Write(b[:])
			}
		case INT64:
			b := int64Bytes(val.AsInt64())
			_, _ = h.Write(b[:])
		case INT64SLICE:
			rv := reflect.ValueOf(val.slice)
			for i := 0; i < rv.Len(); i++ {
				v := rv.Index(i)
				b := int64Bytes(v.Int())
				_, _ = h.Write(b[:])
			}
		case FLOAT64:
			b := float64Bytes(val.AsFloat64())
			_, _ = h.Write(b[:])
		case FLOAT64SLICE:
			rv := reflect.ValueOf(val.slice)
			for i := 0; i < rv.Len(); i++ {
				v := rv.Index(i)
				b := float64Bytes(v.Float())
				_, _ = h.Write(b[:])
			}
		case STRING:
			_, _ = h.Write([]byte(val.AsString()))
		case STRINGSLICE:
			rv := reflect.ValueOf(val.slice)
			for i := 0; i < rv.Len(); i++ {
				v := rv.Index(i)
				_, _ = h.Write([]byte(v.String()))
			}
		}
	}
	return h.Sum64()
}

func boolBytes(val bool) [1]byte {
	if val {
		return [1]byte{1}
	}
	return [1]byte{0}
}

func int64Bytes(val int64) [8]byte {
	// Used for hashing, endianness doesn't matter.
	return [8]byte{
		byte(0xff & val),
		byte(0xff & (val >> 8)),
		byte(0xff & (val >> 16)),
		byte(0xff & (val >> 24)),
		byte(0xff & (val >> 32)),
		byte(0xff & (val >> 40)),
		byte(0xff & (val >> 48)),
		byte(0xff & (val >> 56)),
	}
}

func uint64Bytes(val uint64) [8]byte {
	// Used for hashing, endianness doesn't matter.
	return [8]byte{
		byte(0xff & val),
		byte(0xff & (val >> 8)),
		byte(0xff & (val >> 16)),
		byte(0xff & (val >> 24)),
		byte(0xff & (val >> 32)),
		byte(0xff & (val >> 40)),
		byte(0xff & (val >> 48)),
		byte(0xff & (val >> 56)),
	}
}

func float64Bytes(val float64) [8]byte {
	return uint64Bytes(math.Float64bits(val))
}
