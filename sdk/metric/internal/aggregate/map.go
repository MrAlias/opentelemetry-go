package aggregate

import (
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel/attribute"
)

type cappedMap[N int64 | float64] struct {
	mu sync.Mutex

	// read contains the portion of the map's contents that are safe for
	// concurrent access (with or without mu held).
	//
	// The read field itself is always safe to load, but must only be stored
	// with mu held.
	//
	// Entries stored in read may be updated concurrently without mu, but
	// updating a previously-expunged entry requires that the entry be copied
	// to the dirty map and unexpunged with mu held.
	read atomic.Pointer[readOnly[N]]

	// dirty contains the portion of the map's contents that require mu to be
	// held. To ensure that the dirty map can be promoted to the read map
	// quickly, it also includes all of the non-expunged entries in the read
	// map.
	//
	// Expunged entries are not stored in the dirty map. An expunged entry in
	// the clean map must be unexpunged and added to the dirty map before a new
	// value can be stored to it.
	//
	// If the dirty map is nil, the next write to the map will initialize it by
	// making a shallow copy of the clean map, omitting stale entries.
	dirty map[attribute.Distinct]*entry[N]

	// misses counts the number of loads since the read map was last updated
	// that needed to lock mu to determine whether the key was present.
	//
	// Once enough misses have occurred to cover the cost of copying the dirty
	// map, the dirty map will be promoted to the read map (in the unamended
	// state) and the next store to the map will make a new dirty copy.
	misses int

	// limit is the maximum number of entries allowed in the map.
	limit int
	// size is an atomic counter for the number of entries.
	size int64
	// overflow is called when the map exceeds its limit.
	overflow   func() *sumValue[N]
	overflowed atomic.Bool
}

func newCappedMap[N int64 | float64](limit int, newRes func(attribute.Set) FilteredExemplarReservoir[N]) *cappedMap[N] {
	cm := &cappedMap[N]{
		dirty: make(map[attribute.Distinct]*entry[N]),
		limit: limit,
	}
	cm.overflow = sync.OnceValue(func() *sumValue[N] {
		cm.overflowed.Store(true)
		return &sumValue[N]{attrs: overflowSet, res: newRes(overflowSet)}
	})
	cm.read.Store(&readOnly[N]{m: make(map[attribute.Distinct]*entry[N])})
	return cm
}

// readOnly is an immutable struct that contains the portion of the map that is
// safe for concurrent access without locking.
type readOnly[N int64 | float64] struct {
	m       map[attribute.Distinct]*entry[N]
	amended bool // true if the dirty map contains keys not in m.
}

// An entry is a slot in the map corresponding to a particular key.
type entry[N int64 | float64] struct {
	p atomic.Pointer[sumValue[N]]
}

func newEntry[N int64 | float64](i *sumValue[N]) *entry[N] {
	e := &entry[N]{}
	e.p.Store(i)
	return e
}

func (e *entry[N]) load() (*sumValue[N], bool) {
	p := e.p.Load()
	if p == nil {
		return nil, false
	}
	return p, true
}

func (m *cappedMap[N]) loadReadOnly() readOnly[N] {
	if p := m.read.Load(); p != nil {
		return *p
	}
	return readOnly[N]{}
}

func (m *cappedMap[N]) Len() int {
	n := int(atomic.LoadInt64(&m.size))
	if m.overflowed.Load() {
		return n + 1
	}
	return n
}

// Load retrieves the value for a given key.
// It returns the value and a boolean indicating whether the key was found.
func (m *cappedMap[N]) Load(key attribute.Distinct) (value *sumValue[N], ok bool) {
	read := m.loadReadOnly()
	e, ok := read.m[key]
	if !ok && read.amended {
		m.mu.Lock()

		// Check read map again under lock to avoid reporting a miss if the key
		// was added while we were waiting for the lock.
		read = m.loadReadOnly()
		e, ok = read.m[key]
		if !ok && read.amended {
			e, ok = m.dirty[key]
			// Regardless of whether we find the key, record a miss.
			m.missLocked()
		}
		m.mu.Unlock()
	}
	if !ok {
		return nil, false
	}
	return e.load()
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
//
// If storing the given value would exceed the map's limit, the value is not
// stored and the overflow sumValue is returned instead. In this case, loaded
// will be true.
func (m *cappedMap[N]) LoadOrStore(key attribute.Distinct, value *sumValue[N]) (actual *sumValue[N], loaded bool) {
	// Fast path - check read map first
	read := m.loadReadOnly()
	if e, ok := read.m[key]; ok {
		return e.loadOrStore(value)
	}

	// Slow path - lock and check again

	m.mu.Lock()
	read = m.loadReadOnly()
	if e, ok := read.m[key]; ok {
		actual, loaded = e.loadOrStore(value)
	} else if e, ok := m.dirty[key]; ok {
		actual, loaded = e.loadOrStore(value)
		m.missLocked()
	} else if n := atomic.LoadInt64(&m.size); int(n) >= m.limit-1 {
		actual, loaded = m.overflow(), true
	} else {
		if !read.amended {
			// First new key to the dirty map. Make sure it is allocated and
			// mark the read-only map as amended.
			m.dirtyLocked()
			m.read.Store(&readOnly[N]{m: read.m, amended: true})
		}
		m.dirty[key] = newEntry(value)
		atomic.AddInt64(&m.size, 1)
		actual, loaded = value, false
	}
	m.mu.Unlock()

	return actual, loaded
}

// loadOrStore atomically loads or stores a value.
func (e *entry[N]) loadOrStore(value *sumValue[N]) (actual *sumValue[N], loaded bool) {
	p := e.p.Load()
	if p != nil {
		return p, true
	}

	for {
		if e.p.CompareAndSwap(nil, value) {
			return value, false
		}
		p = e.p.Load()
		if p != nil {
			return p, true
		}
	}
}

// dirtyLocked copies the read map to dirty - called with mutex held
func (m *cappedMap[N]) dirtyLocked() {
	if m.dirty != nil {
		return
	}

	read := m.loadReadOnly()
	m.dirty = make(map[attribute.Distinct]*entry[N], len(read.m))
	for k, v := range read.m {
		m.dirty[k] = v
	}
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
//
// Range does not necessarily correspond to any consistent snapshot of the Map's
// contents: no key will be visited more than once, but if the value for any key
// is stored or deleted concurrently (including by f), Range may reflect any
// mapping for that key from any point during the Range call. Range does not
// block other methods on the receiver; even f itself may call any method on m.
//
// Range may be O(N) with the number of elements in the map even if f returns
// false after a constant number of calls.
func (m *cappedMap[N]) Range(f func(key attribute.Distinct, value *sumValue[N]) bool) {
	// We need to be able to iterate over all of the keys that were already
	// present at the start of the call to Range.
	// If read.amended is false, then read.m satisfies that property without
	// requiring us to hold m.mu for a long time.
	read := m.loadReadOnly()
	if read.amended {
		// m.dirty contains keys not in read.m.
		m.mu.Lock()
		read = m.loadReadOnly()
		if read.amended {
			read = readOnly[N]{m: m.dirty}
			copyRead := read
			m.read.Store(&copyRead)
			m.dirty = nil
			m.misses = 0
		}
		m.mu.Unlock()
	}

	for k, e := range read.m {
		v, ok := e.load()
		if !ok {
			continue
		}
		if !f(k, v) {
			break
		}
	}
	if m.overflowed.Load() {
		// If we have overflowed, we need to call f with the overflow
		// value as well.
		f(overflowSet.Equivalent(), m.overflow())
	}
}

// Clear deletes all the entries, resulting in an empty Map.
func (m *cappedMap[N]) Clear() {
	read := m.loadReadOnly()
	if len(read.m) == 0 && !read.amended {
		// Avoid allocation if already empty.
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	read = m.loadReadOnly()
	if len(read.m) > 0 || read.amended {
		m.read.Store(&readOnly[N]{})
	}

	clear(m.dirty)
	m.misses = 0
	atomic.StoreInt64(&m.size, 0)
	m.overflowed.Store(false)
}

// missLocked handles a miss in the read map - called with mutex held
func (m *cappedMap[N]) missLocked() {
	m.misses++
	if m.misses < len(m.dirty) {
		return
	}

	// Promote dirty to read
	m.read.Store(&readOnly[N]{m: m.dirty})
	m.dirty = nil
	m.misses = 0
}

type atomicSyncMap[N int64 | float64] struct {
	m    sync.Map // map[attribute.Distinct]*sumValue[N]
	size int64    // atomic counter for the number of entries.
}

func newAtomicSyncMap[N int64 | float64]() atomicMap[N] {
	return &atomicSyncMap[N]{}
}

func (m *atomicSyncMap[N]) Len() int {
	return int(atomic.LoadInt64(&m.size))
}

func (m *atomicSyncMap[N]) LoadOrStore(key attribute.Distinct, value *sumValue[N]) (actual *sumValue[N], loaded bool) {
	v, loaded := m.m.LoadOrStore(key, value)
	if !loaded {
		atomic.AddInt64(&m.size, 1)
	}
	return v.(*sumValue[N]), loaded
}

func (m *atomicSyncMap[N]) Range(f func(key attribute.Distinct, value *sumValue[N]) bool) {
	m.m.Range(func(k, v any) bool {
		return f(k.(attribute.Distinct), v.(*sumValue[N]))
	})
}

func (m *atomicSyncMap[N]) Clear() {
	m.m.Clear()
	atomic.StoreInt64(&m.size, 0)
}
