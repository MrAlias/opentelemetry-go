package attribute_test

import (
	"fmt"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

var result attribute.Distinct

func BenchmarkNewDistinct(b *testing.B) {
	all := []attribute.KeyValue{
		attribute.Bool("bool true", true),
		attribute.Bool("bool false", false),
		attribute.Int64("int64 1", 1),
		attribute.Int64("int64 -1", -1),
		attribute.Float64("float64 10", 10),
		attribute.Float64("float64 -10", -10),
		attribute.String("string empty", ""),
		attribute.String("hello", "world"),
		attribute.BoolSlice("[]bool", []bool{true, false, true}),
		attribute.Int64Slice("[]int64", []int64{-10, 13124, -23, 0, 2}),
		attribute.Float64Slice("[]float64", []float64{10.23, 941.1, 184e9, -2.3}),
		attribute.StringSlice("[]string", []string{"", "one", "two"}),
	}

	kvs := all[:2]
	b.Log(kvsPrettyPrint(kvs))
	b.Run("Bool", benchmarkNewDistinct(kvs))

	kvs = all[2:4]
	b.Log(kvsPrettyPrint(kvs))
	b.Run("Int64", benchmarkNewDistinct(kvs))

	kvs = all[4:6]
	b.Log(kvsPrettyPrint(kvs))
	b.Run("Float64", benchmarkNewDistinct(kvs))

	kvs = all[6:8]
	b.Log(kvsPrettyPrint(kvs))
	b.Run("String", benchmarkNewDistinct(kvs))

	kvs = all[8:9]
	b.Log(kvsPrettyPrint(kvs))
	b.Run("BoolSlice", benchmarkNewDistinct(kvs))

	kvs = all[9:10]
	b.Log(kvsPrettyPrint(kvs))
	b.Run("Int64Slice", benchmarkNewDistinct(kvs))

	kvs = all[10:11]
	b.Log(kvsPrettyPrint(kvs))
	b.Run("Float64Slice", benchmarkNewDistinct(kvs))

	kvs = all[11:12]
	b.Log(kvsPrettyPrint(kvs))
	b.Run("StringSlice", benchmarkNewDistinct(kvs))

	b.Log(kvsPrettyPrint(all))
	b.Run("All", benchmarkNewDistinct(all))
}

type kvsPrettyPrint []attribute.KeyValue

func (a kvsPrettyPrint) String() string {
	out := make([]string, 0, len(a))
	for _, kv := range a {
		item := fmt.Sprintf("%q: %s", kv.Key, kv.Value.Emit())
		out = append(out, item)
	}
	return "[" + strings.Join(out, ",") + "]"
}

func benchmarkNewDistinct(kvs []attribute.KeyValue) func(b *testing.B) {
	return func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			result = attribute.NewDistinct(kvs)
		}
	}
}

var outN int

func BenchmarkDistinctMapKey(b *testing.B) {
	const n = 1024
	keys := make([]attribute.Distinct, n)
	for i := range keys {
		kvs := []attribute.KeyValue{attribute.Int("key", i)}
		keys[i] = attribute.NewDistinct(kvs)
	}

	m := make(map[attribute.Distinct]int, n)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range keys {
			m[keys[j]] = i
		}

		for j := range keys {
			outN = m[keys[j]]
		}
	}
}
