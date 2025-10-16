// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oops

import (
	"errors"
	"fmt"
	"math"
	"testing"
)

func TestErrPartialAsImplementation(t *testing.T) {
	err := ErrPartial{}
	if _, ok := any(err).(interface{ As(any) bool }); !ok {
		t.Fatal("ErrPartial does not implement errors.As method")
	}
}

func TestErrPartialIsImplementation(t *testing.T) {
	err := ErrPartial{}
	if _, ok := any(err).(interface{ Is(error) bool }); !ok {
		t.Fatal("ErrPartial does not implement errors.Is method")
	}
}

var loremIpsum = `
Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor 
incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis 
nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. 
Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore 
eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt 
in culpa qui officia deserunt mollit anim id est laborum.
`

func TestErrPartialError(t *testing.T) {
	tests := []struct {
		n   int64
		msg string
	}{
		{0, ""},
		{0, "all good"},
		{1, "partial success"},
		{1234567890, "partial success"},
		{1234567890, ""},
		{math.MaxInt64, "max int64"},
		{math.MinInt64, "min int64"}, // negative number, should not panic.
		{0, loremIpsum},              // long message.
	}
	for _, tt := range tests {
		err := ErrPartial{Rejected: tt.n, Message: tt.msg}
		var want string
		if tt.msg == "" {
			format := "OTLP partial upload success: %d spans rejected"
			want = fmt.Sprintf(format, tt.n)
		} else {
			format := "OTLP partial upload success: %s: %d spans rejected"
			want = fmt.Sprintf(format, tt.msg, tt.n)
		}
		t.Log("Testing:", want)
		if got := err.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	}
}

func BenchmarkErrPartial(b *testing.B) {
	err := ErrPartial{Rejected: 123, Message: "partial success"}

	b.Run("Error", func(b *testing.B) {
		var msg string
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			msg = err.Error()
		}
		_ = msg
	})

	b.Run("As", func(b *testing.B) {
		target := new(ErrPartial)
		var ok bool
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			ok = err.As(target)
		}
		_ = ok
	})

	b.Run("Is", func(b *testing.B) {
		var target error = ErrPartial{}
		var ok bool
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			ok = err.Is(target)
		}
		_ = ok
	})
}

func TestErrPartialAs(t *testing.T) {
	partialErr := ErrPartial{Rejected: 42, Message: "test message"}

	t.Run("successful assignment", func(t *testing.T) {
		var target ErrPartial
		ok := partialErr.As(&target)
		if !ok {
			t.Fatal("As() returned false, want true")
		}
		if target.Rejected != 42 {
			t.Errorf("target.Rejected = %d, want 42", target.Rejected)
		}
		if target.Message != "test message" {
			t.Errorf("target.Message = %q, want %q", target.Message, "test message")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		var target ErrRPC
		ok := partialErr.As(&target)
		if ok {
			t.Error("As() returned true, want false")
		}
	})

	t.Run("not a pointer", func(t *testing.T) {
		var target ErrPartial
		ok := partialErr.As(target)
		if ok {
			t.Error("As() returned true, want false")
		}
	})

	t.Run("nil target", func(t *testing.T) {
		ok := partialErr.As(nil)
		if ok {
			t.Error("As() returned true, want false")
		}
	})
}

func TestErrPartialIs(t *testing.T) {
	tests := []struct {
		name   string
		target ErrPartial
		err    error
		want   bool
	}{
		{
			name:   "same type",
			target: ErrPartial{Rejected: 1, Message: "msg1"},
			err:    ErrPartial{Rejected: 2, Message: "msg2"},
			want:   true,
		},
		{
			name:   "same values",
			target: ErrPartial{Rejected: 5, Message: "same"},
			err:    ErrPartial{Rejected: 5, Message: "same"},
			want:   true,
		},
		{
			name:   "different type",
			target: ErrPartial{Rejected: 1, Message: "msg"},
			err:    errors.New("generic error"),
			want:   false,
		},
		{
			name:   "wrapped ErrPartial",
			target: ErrPartial{Rejected: 1, Message: "msg"},
			err:    fmt.Errorf("wrapped: %w", ErrPartial{Rejected: 3, Message: "inner"}),
			want:   false, // The Is method only checks direct type assertion, not wrapped errors
		},
		{
			name:   "ErrRPC",
			target: ErrPartial{Rejected: 1, Message: "msg"},
			err:    ErrRPC{},
			want:   false,
		},
		{
			name:   "ErrWarn",
			target: ErrPartial{Rejected: 1, Message: "msg"},
			err:    ErrWarn{},
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

func TestErrPartialErrorType(t *testing.T) {
	tests := []struct {
		name     string
		rejected int64
		message  string
		want     string
	}{
		{
			name:     "any values return PartialSuccess",
			rejected: 5,
			message:  "test message",
			want:     "PartialSuccess",
		},
		{
			name:     "zero rejected returns PartialSuccess",
			rejected: 0,
			message:  "test",
			want:     "PartialSuccess",
		},
		{
			name:     "empty message returns PartialSuccess",
			rejected: 10,
			message:  "",
			want:     "PartialSuccess",
		},
		{
			name:     "large rejected count returns PartialSuccess",
			rejected: math.MaxInt64,
			message:  "large count",
			want:     "PartialSuccess",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ErrPartial{Rejected: tt.rejected, Message: tt.message}
			got := err.ErrorType()
			if got != tt.want {
				t.Errorf("ErrorType() = %q, want %q", got, tt.want)
			}
		})
	}
}
