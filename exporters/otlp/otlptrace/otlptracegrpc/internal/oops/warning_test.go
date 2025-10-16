// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oops

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrWarnAsImplementation(t *testing.T) {
	err := ErrWarn{}
	if _, ok := any(err).(interface{ As(any) bool }); !ok {
		t.Fatal("ErrWarn does not implement errors.As method")
	}
}

func TestErrWarnIsImplementation(t *testing.T) {
	err := ErrWarn{}
	if _, ok := any(err).(interface{ Is(error) bool }); !ok {
		t.Fatal("ErrWarn does not implement errors.Is method")
	}
}

func TestErrWarnError(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "simple message",
			message: "deprecated field used",
			want:    "OTLP warning: deprecated field used",
		},
		{
			name:    "empty message",
			message: "",
			want:    "OTLP warning: ",
		},
		{
			name:    "long message",
			message: "this is a very long warning message that contains multiple words and punctuation marks!",
			want:    "OTLP warning: this is a very long warning message that contains multiple words and punctuation marks!",
		},
		{
			name:    "message with newlines",
			message: "line 1\nline 2\nline 3",
			want:    "OTLP warning: line 1\nline 2\nline 3",
		},
		{
			name:    "message with special characters",
			message: "special chars: 123 !@#$%^&*()_+-=[]{}|;':\",./<>?",
			want:    "OTLP warning: special chars: 123 !@#$%^&*()_+-=[]{}|;':\",./<>?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ErrWarn{Message: tt.message}
			got := err.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrWarnAs(t *testing.T) {
	warnErr := ErrWarn{Message: "test warning"}

	t.Run("successful assignment", func(t *testing.T) {
		var target ErrWarn
		ok := warnErr.As(&target)
		if !ok {
			t.Fatal("As() returned false, want true")
		}
		if target.Message != "test warning" {
			t.Errorf("target.Message = %q, want %q", target.Message, "test warning")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		var target ErrRPC
		ok := warnErr.As(&target)
		if ok {
			t.Error("As() returned true, want false")
		}
	})

	t.Run("not a pointer", func(t *testing.T) {
		var target ErrWarn
		ok := warnErr.As(target)
		if ok {
			t.Error("As() returned true, want false")
		}
	})

	t.Run("nil target", func(t *testing.T) {
		ok := warnErr.As(nil)
		if ok {
			t.Error("As() returned true, want false")
		}
	})
}

func TestErrWarnIs(t *testing.T) {
	tests := []struct {
		name   string
		target ErrWarn
		err    error
		want   bool
	}{
		{
			name:   "same type",
			target: ErrWarn{Message: "warning1"},
			err:    ErrWarn{Message: "warning2"},
			want:   true,
		},
		{
			name:   "same message",
			target: ErrWarn{Message: "same"},
			err:    ErrWarn{Message: "same"},
			want:   true,
		},
		{
			name:   "different type",
			target: ErrWarn{Message: "warning"},
			err:    errors.New("generic error"),
			want:   false,
		},
		{
			name:   "wrapped ErrWarn",
			target: ErrWarn{Message: "warning"},
			err:    fmt.Errorf("wrapped: %w", ErrWarn{Message: "inner warning"}),
			want:   false, // The Is method only checks direct type assertion, not wrapped errors
		},
		{
			name:   "ErrRPC",
			target: ErrWarn{Message: "warning"},
			err:    ErrRPC{},
			want:   false,
		},
		{
			name:   "ErrPartial",
			target: ErrWarn{Message: "warning"},
			err:    ErrPartial{},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.target.Is(tt.err)
			if got != tt.want {
				t.Errorf("Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrWarnErrorType(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "any message returns Warning",
			message: "test message",
			want:    "Warning",
		},
		{
			name:    "empty message returns Warning",
			message: "",
			want:    "Warning",
		},
		{
			name:    "long message returns Warning",
			message: "this is a very long message that should still return Warning",
			want:    "Warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ErrWarn{Message: tt.message}
			got := err.ErrorType()
			if got != tt.want {
				t.Errorf("ErrorType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func BenchmarkErrWarn(b *testing.B) {
	err := ErrWarn{Message: "test warning message"}

	b.Run("Error", func(b *testing.B) {
		var msg string
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			msg = err.Error()
		}
		_ = msg
	})

	b.Run("ErrorType", func(b *testing.B) {
		var typ string
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			typ = err.ErrorType()
		}
		_ = typ
	})

	b.Run("As", func(b *testing.B) {
		target := new(ErrWarn)
		var ok bool
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ok = err.As(target)
		}
		_ = ok
	})

	b.Run("Is", func(b *testing.B) {
		var target error = ErrWarn{}
		var ok bool
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ok = err.Is(target)
		}
		_ = ok
	})
}
