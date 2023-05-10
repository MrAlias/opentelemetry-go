package attribute

import (
	"fmt"
	"reflect"
	"sort"

	"go.opentelemetry.io/otel/attribute/internal/fnv"
)

// Collection is a group of KeyValues.
type Collection struct {
	// Only one of set or data should be defined at a time. Both are used so
	// lazy evaluate can be performed while still supporting construction from
	// both a Set and []KeyValue.
	set *Set
	// [...]KeyValue
	data reflect.Value

	// droppedIdx is the index of data from which point all the remaining
	// attributes are the ones dropped by filtering.
	droppedIdx int

	hash Distinct

	// Ensure users do not use this as a map key.
	noCmp [0]func()
}

func NewCollection(kvs ...KeyValue) *Collection {
	if len(kvs) == 0 {
		return &Collection{set: emptySet}
	}

	// Copy into a fixed size array so...
	//  - the underlying data is not changed outside the returned Collection
	//  - access to the underlying data via reflect (i.e. using the
	//    Interface() method) does not allocate
	data := computeData(kvs)
	return &Collection{data: reflect.ValueOf(data), droppedIdx: len(kvs)}
}

func (c *Collection) Distinct() Distinct {
	if c.hash.Valid() {
		// Return existing computation of Distinct.
		return c.hash
	}

	switch {
	case c.data.IsValid():
		c.hash = c.distinct()
	default:
		// Default to the empty set Distinct if c.set unset.
		c.hash = c.set.Equivalent()
	}

	return c.hash
}

func (c *Collection) distinct() Distinct {
	h := fnv.New()
	for i := 0; i < c.droppedIdx; i++ {
		// Given the underlying storage is an array, the modern Go compiler is
		// able to avoid the interface{} allocation here.
		kv := c.data.Index(i).Interface().(KeyValue)

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

// Filter applies the Filter f to the collection in place. If the collection
// was modified by the filter true is returned, otherwise false.
func (c *Collection) Filter(f Filter) bool {
	var filtered bool
	switch {
	case c.set != nil:
		// c.set cannot be mutated. Make a copy to c.data.
		c.data = reflect.New(reflect.ArrayOf(c.set.Len(), keyValueType)).Elem()
		reflect.Copy(c.data, c.set.reflectValue())
		c.hash = c.set.distinct
		c.set = nil
		fallthrough
	case c.data.IsValid():
		filtered = c.filter(f)
	}

	if filtered {
		// Reset c.distinct to invalid so it is recomputed.
		c.hash = Distinct{}
	}

	return filtered
}

// filter applies the Filter f to c.slice. All dropped KeyValues are moved to
// the end of the slice and c.droppedIdx is updated accordingly.
func (c *Collection) filter(f Filter) bool {
	start := c.droppedIdx
	for i := 0; i < c.droppedIdx; i++ {
		kv := c.data.Index(i)
		if f(kv.Interface().(KeyValue)) {
			continue
		}

		// Swap kv to the dopped region. Order is not preserved.
		//
		//                          droppedIdx (start)
		//  ___ ___     ___  ___     ___ _↓_ ___     ___
		// |   |   |   |   ||   |   |   |   |   |   |   |
		// | 0 | 1 |...| i ||i+1|...| j | d |d+1|...| n |
		// |___|___|   |___||___|   |___|___|___|   |___|
		//                _\ ________/
		//               |  \_________
		//  ___ ___     _↓_  ___     _↓_ ___ ___     ___
		// |   |   |   |   ||   |   |   |   |   |   |   |
		// | 0 | 1 |...| j ||i+1|...| i | d |d+1|...| n |
		// |___|___|   |___||___|   |___|___|___|   |___|
		//                            ↑
		//                       droppedIdx (end)
		c.droppedIdx--
		c.data.Index(i).Set(c.data.Index(c.droppedIdx))
		c.data.Index(c.droppedIdx).Set(kv)
	}
	return c.droppedIdx < start
}

// Dropped returns the attributes filtered out of the collection.
func (c *Collection) Dropped() []KeyValue {
	if !c.data.IsValid() || c.droppedIdx == c.data.Len() {
		// No filtering of the underlying set or slice has been done.
		return nil
	}
	n := c.data.Len() - c.droppedIdx
	cp := make([]KeyValue, n)
	reflect.Copy(reflect.ValueOf(cp), c.data)
	return cp
}

// CopyDropped copies the dropped KeyValues into dest. This will panic if dest
// is nil.
func (c *Collection) CopyDropped(dest *[]KeyValue) {
	if !c.data.IsValid() || c.droppedIdx == c.data.Len() {
		// No filtering of the underlying set or slice has been done.
		*dest = (*dest)[:0]
		return
	}

	n := c.data.Len() - c.droppedIdx
	if cap(*dest) < n {
		*dest = make([]KeyValue, n)
	}
	reflect.Copy(reflect.ValueOf(*dest), c.data)
	*dest = (*dest)[:n]
}

func (c *Collection) Iter() Iterator {
	if c.set != nil {
		return c.set.Iter()
	}

	if !c.data.IsValid() {
		return Iterator{
			storage: reflect.ValueOf(emptySet.data),
			idx:     -1,
		}
	}

	// Iterators are sorted by key.
	d := c.data.Slice(0, c.droppedIdx).Interface()
	sort.SliceStable(d, func(i, j int) bool {
		kvI := c.data.Index(i).Interface().(KeyValue)
		kvJ := c.data.Index(j).Interface().(KeyValue)
		return kvI.Key < kvJ.Key
	})

	return Iterator{
		storage: reflect.ValueOf(d).Elem(),
		idx:     -1,
	}
}
