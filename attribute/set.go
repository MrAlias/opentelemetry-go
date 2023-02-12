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
	"encoding/json"
	"reflect"
	"sort"
)

type (
	// Set is an immutable collection of distinct attributes.
	//
	// A set should not be used as a map key or compared directly. The
	// Equivalent method returns a Distinct that should be used for comparison
	// of the Set value.
	Set struct {
		store    storage
		distinct Distinct
	}

	storage struct {
		data interface{}
	}

	// Filter supports removing certain attributes from attribute sets. When
	// the filter returns true, the attribute will be kept in the filtered
	// attribute set. When the filter returns false, the attribute is excluded
	// from the filtered attribute set, and the attribute instead appears in
	// the removed list of excluded attributes.
	Filter func(KeyValue) bool

	// Sortable implements sort.Interface, used for sorting KeyValue. This is
	// an exported type to support a memory optimization. A pointer to one of
	// these is needed for the call to sort.Stable(), which the caller may
	// provide in order to avoid an allocation. See NewSetWithSortable().
	Sortable []KeyValue
)

// Compile-time check that a Set remains a valid map key type (even though it
// should not be used this way).
var _ map[Set]struct{} = nil

var (
	// keyValueType is used in computeDistinctReflect.
	keyValueType = reflect.TypeOf(KeyValue{})

	emptyStore = newStorage(nil)
	// emptySet is returned for empty attribute sets.
	emptySet = &Set{
		store:    *emptyStore,
		distinct: Distinct{valid: true},
	}
)

// EmptySet returns a reference to a Set with no elements.
//
// This is a convenience provided for optimized calling utility.
func EmptySet() *Set {
	return emptySet
}

func (s storage) reflectValue() reflect.Value {
	return reflect.ValueOf(s.data)
}

func (s storage) len() int {
	if s.data == nil {
		return 0
	}
	return s.reflectValue().Len()
}

// getAt returns the indexed KeyValue at idx.
func (s storage) getAt(idx int) (KeyValue, bool) {
	if s.data == nil {
		return KeyValue{}, false
	}
	value := s.reflectValue()

	if idx >= 0 && idx < value.Len() {
		// Note: The Go compiler successfully avoids an allocation for
		// the interface{} conversion here:
		return value.Index(idx).Interface().(KeyValue), true
	}

	return KeyValue{}, false
}

// get returns the Value in storage stored with k.
func (s storage) get(k Key) (Value, bool) {
	if s.data == nil {
		return Value{}, false
	}
	rValue := s.reflectValue()
	vlen := rValue.Len()

	idx := sort.Search(vlen, func(idx int) bool {
		return rValue.Index(idx).Interface().(KeyValue).Key >= k
	})
	if idx >= vlen {
		return Value{}, false
	}
	keyValue := rValue.Index(idx).Interface().(KeyValue)
	if k == keyValue.Key {
		return keyValue.Value, true
	}
	return Value{}, false
}

// Len returns the number of attributes in this set.
func (s *Set) Len() int {
	if s == nil {
		return 0
	}
	return s.store.len()
}

// Get returns the KeyValue at ordered position idx in this set.
func (s *Set) Get(idx int) (KeyValue, bool) {
	if s == nil {
		return KeyValue{}, false
	}
	return s.store.getAt(idx)
}

// Value returns the value of a specified key in this set.
func (s *Set) Value(k Key) (Value, bool) {
	if s == nil {
		return Value{}, false
	}
	return s.store.get(k)
}

// HasValue tests whether a key is defined in this set.
func (s *Set) HasValue(k Key) bool {
	if s == nil {
		return false
	}
	_, ok := s.store.get(k)
	return ok
}

// Iter returns an iterator for visiting the attributes in this set.
func (s *Set) Iter() Iterator {
	return newIterator(&s.store)
}

// ToSlice returns the set of attributes belonging to this set, sorted, where
// keys appear no more than once.
func (s *Set) ToSlice() []KeyValue {
	iter := s.Iter()
	return iter.ToSlice()
}

// Equivalent returns a value that may be used as a map key. The Distinct type
// guarantees that the result will equal the equivalent. Distinct value of any
// attribute set with the same elements as this, where sets are made unique by
// choosing the last value in the input for any given key.
func (s *Set) Equivalent() Distinct {
	if s == nil || !s.distinct.Valid() {
		return emptySet.distinct
	}
	return s.distinct
}

// Equals returns true if the argument set is equivalent to this set.
func (s *Set) Equals(o *Set) bool {
	return s.Equivalent() == o.Equivalent()
}

// Encoded returns the encoded form of this set, according to encoder.
func (s *Set) Encoded(encoder Encoder) string {
	if s == nil || encoder == nil {
		return ""
	}

	return encoder.Encode(s.Iter())
}

// NewSet returns a new Set. See the documentation for
// NewSetWithSortableFiltered for more details.
//
// Except for empty sets, this method adds an additional allocation compared
// with calls that include a Sortable.
func NewSet(kvs ...KeyValue) Set {
	// Check for empty set.
	if len(kvs) == 0 {
		return *EmptySet()
	}
	s, _ := NewSetWithSortableFiltered(kvs, new(Sortable), nil)
	return s
}

// NewSetWithSortable returns a new Set. See the documentation for
// NewSetWithSortableFiltered for more details.
//
// This call includes a Sortable option as a memory optimization.
func NewSetWithSortable(kvs []KeyValue, tmp *Sortable) Set {
	// Check for empty set.
	if len(kvs) == 0 {
		return *EmptySet()
	}
	s, _ := NewSetWithSortableFiltered(kvs, tmp, nil)
	return s
}

// NewSetWithFiltered returns a new Set. See the documentation for
// NewSetWithSortableFiltered for more details.
//
// This call includes a Filter to include/exclude attribute keys from the
// return value. Excluded keys are returned as a slice of attribute values.
func NewSetWithFiltered(kvs []KeyValue, filter Filter) (Set, []KeyValue) {
	// Check for empty set.
	if len(kvs) == 0 {
		return *EmptySet(), nil
	}
	return NewSetWithSortableFiltered(kvs, new(Sortable), filter)
}

// NewSetWithSortableFiltered returns a new Set.
//
// Duplicate keys are eliminated by taking the last value.  This
// re-orders the input slice so that unique last-values are contiguous
// at the end of the slice.
//
// This ensures the following:
//
// - Last-value-wins semantics
// - Caller sees the reordering, but doesn't lose values
// - Repeated call preserve last-value wins.
//
// Note that methods are defined on Set, although this returns Set. Callers
// can avoid memory allocations by:
//
// - allocating a Sortable for use as a temporary in this method
// - allocating a Set for storing the return value of this constructor.
//
// The result maintains a cache of encoded attributes, by attribute.EncoderID.
// This value should not be copied after its first use.
//
// The second []KeyValue return value is a list of attributes that were
// excluded by the Filter (if non-nil).
func NewSetWithSortableFiltered(kvs []KeyValue, tmp *Sortable, filter Filter) (Set, []KeyValue) {
	// Check for empty set.
	if len(kvs) == 0 {
		return *EmptySet(), nil
	}

	*tmp = kvs

	// Stable sort so the following de-duplication can implement
	// last-value-wins semantics.
	sort.Stable(tmp)

	*tmp = nil

	position := len(kvs) - 1
	offset := position - 1

	// The requirements stated above require that the stable
	// result be placed in the end of the input slice, while
	// overwritten values are swapped to the beginning.
	//
	// De-duplicate with last-value-wins semantics.  Preserve
	// duplicate values at the beginning of the input slice.
	for ; offset >= 0; offset-- {
		if kvs[offset].Key == kvs[position].Key {
			continue
		}
		position--
		kvs[offset], kvs[position] = kvs[position], kvs[offset]
	}
	if filter != nil {
		return filterSet(kvs[position:], filter)
	}
	store := newStorage(kvs[position:])
	return Set{
		store:    *store,
		distinct: newDistinct(newIterator(store)),
	}, nil
}

// filterSet reorders kvs so that included keys are contiguous at the end of
// the slice, while excluded keys precede the included keys.
func filterSet(kvs []KeyValue, filter Filter) (Set, []KeyValue) {
	var excluded []KeyValue

	// Move attributes that do not match the filter so they're adjacent before
	// calling computeDistinct().
	distinctPosition := len(kvs)

	// Swap indistinct keys forward and distinct keys toward the
	// end of the slice.
	offset := len(kvs) - 1
	for ; offset >= 0; offset-- {
		if filter(kvs[offset]) {
			distinctPosition--
			kvs[offset], kvs[distinctPosition] = kvs[distinctPosition], kvs[offset]
			continue
		}
	}
	excluded = kvs[:distinctPosition]

	store := newStorage(kvs[distinctPosition:])
	return Set{
		store:    *store,
		distinct: newDistinct(newIterator(store)),
	}, excluded
}

// Filter returns a filtered copy of this Set. See the documentation for
// NewSetWithSortableFiltered for more details.
func (s *Set) Filter(re Filter) (Set, []KeyValue) {
	if re == nil {
		return Set{
			distinct: s.distinct,
		}, nil
	}

	// Note: This could be refactored to avoid the temporary slice
	// allocation, if it proves to be expensive.
	return filterSet(s.ToSlice(), re)
}

// newStorage returns kvs as an interface using either the fixed- or
// reflect-oriented code path, depending on the size of the input. The input
// slice is assumed to already be sorted and de-duplicated.
func newStorage(kvs []KeyValue) *storage {
	iface := ifaceDataFixed(kvs)
	if iface == nil {
		iface = ifaceDataReflect(kvs)
	}
	return &storage{data: iface}
}

// ifaceDataFixed returns kvs as an interface for small slices. It returns nil
// if the input is too large for this code path.
func ifaceDataFixed(kvs []KeyValue) interface{} {
	switch len(kvs) {
	case 0:
		return [0]KeyValue{}
	case 1:
		ptr := new([1]KeyValue)
		copy((*ptr)[:], kvs)
		return *ptr
	case 2:
		ptr := new([2]KeyValue)
		copy((*ptr)[:], kvs)
		return *ptr
	case 3:
		ptr := new([3]KeyValue)
		copy((*ptr)[:], kvs)
		return *ptr
	case 4:
		ptr := new([4]KeyValue)
		copy((*ptr)[:], kvs)
		return *ptr
	case 5:
		ptr := new([5]KeyValue)
		copy((*ptr)[:], kvs)
		return *ptr
	case 6:
		ptr := new([6]KeyValue)
		copy((*ptr)[:], kvs)
		return *ptr
	case 7:
		ptr := new([7]KeyValue)
		copy((*ptr)[:], kvs)
		return *ptr
	case 8:
		ptr := new([8]KeyValue)
		copy((*ptr)[:], kvs)
		return *ptr
	case 9:
		ptr := new([9]KeyValue)
		copy((*ptr)[:], kvs)
		return *ptr
	case 10:
		ptr := new([10]KeyValue)
		copy((*ptr)[:], kvs)
		return *ptr
	default:
		return nil
	}
}

// ifaceDataReflect return kvs as an interface{} using reflection, works for
// any size input.
func ifaceDataReflect(kvs []KeyValue) interface{} {
	at := reflect.New(reflect.ArrayOf(len(kvs), keyValueType)).Elem()
	for i, keyValue := range kvs {
		*(at.Index(i).Addr().Interface().(*KeyValue)) = keyValue
	}
	return at.Interface()
}

// MarshalJSON returns the JSON encoding of the Set.
func (s *Set) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.store.data)
}

// MarshalLog is the marshaling function used by the logging system to represent this exporter.
func (s Set) MarshalLog() interface{} {
	kvs := make(map[string]string)
	for _, kv := range s.ToSlice() {
		kvs[string(kv.Key)] = kv.Value.Emit()
	}
	return kvs
}

// Len implements sort.Interface.
func (l *Sortable) Len() int {
	return len(*l)
}

// Swap implements sort.Interface.
func (l *Sortable) Swap(i, j int) {
	(*l)[i], (*l)[j] = (*l)[j], (*l)[i]
}

// Less implements sort.Interface.
func (l *Sortable) Less(i, j int) bool {
	return (*l)[i].Key < (*l)[j].Key
}
