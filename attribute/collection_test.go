package attribute

import "testing"

var result Distinct

func BenchmarkCollectionDistinct(b *testing.B) {
	c := NewCollection(
		Bool("bool true", true),
		Bool("bool false", false),
		Int64("int64 1", 1),
		Int64("int64 -1", -1),
		Float64("float64 10", 10),
		Float64("float64 -10", -10),
		String("string empty", ""),
		String("hello", "world"),
		BoolSlice("[]bool", []bool{true, false, true}),
		Int64Slice("[]int64", []int64{-10, 13124, -23, 0, 2}),
		Float64Slice("[]float64", []float64{10.23, 941.1, 184e9, -2.3}),
		StringSlice("[]string", []string{"", "one", "two"}),
	)

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		result = c.distinct()
	}
}
