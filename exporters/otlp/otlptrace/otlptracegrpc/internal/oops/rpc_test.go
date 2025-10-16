// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oops

import (
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestErrRPCAsImplementation(t *testing.T) {
	err := ErrRPC{}
	if _, ok := any(err).(interface{ As(any) bool }); !ok {
		t.Fatal("ErrRPC does not implement errors.As method")
	}
}

func TestErrRPCIsImplementation(t *testing.T) {
	err := ErrRPC{}
	if _, ok := any(err).(interface{ Is(error) bool }); !ok {
		t.Fatal("ErrRPC does not implement errors.Is method")
	}
}

func TestErrRPCUnwrapImplementation(t *testing.T) {
	err := ErrRPC{}
	if _, ok := any(err).(interface{ Unwrap() error }); !ok {
		t.Fatal("ErrRPC does not implement errors.Unwrap method")
	}
}

func TestErrRPCErrorType(t *testing.T) {
	tests := []struct {
		name string
		code codes.Code
		want string
	}{
		{"OK", codes.OK, "OK"},
		{"Canceled", codes.Canceled, "Canceled"},
		{"Unknown", codes.Unknown, "Unknown"},
		{"InvalidArgument", codes.InvalidArgument, "InvalidArgument"},
		{"DeadlineExceeded", codes.DeadlineExceeded, "DeadlineExceeded"},
		{"NotFound", codes.NotFound, "NotFound"},
		{"AlreadyExists", codes.AlreadyExists, "AlreadyExists"},
		{"PermissionDenied", codes.PermissionDenied, "PermissionDenied"},
		{"ResourceExhausted", codes.ResourceExhausted, "ResourceExhausted"},
		{"FailedPrecondition", codes.FailedPrecondition, "FailedPrecondition"},
		{"Aborted", codes.Aborted, "Aborted"},
		{"OutOfRange", codes.OutOfRange, "OutOfRange"},
		{"Unimplemented", codes.Unimplemented, "Unimplemented"},
		{"Internal", codes.Internal, "Internal"},
		{"Unavailable", codes.Unavailable, "Unavailable"},
		{"DataLoss", codes.DataLoss, "DataLoss"},
		{"Unauthenticated", codes.Unauthenticated, "Unauthenticated"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ErrRPC{Code: tt.code}
			got := err.ErrorType()
			if got != tt.want {
				t.Errorf("ErrorType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestErrRPCError(t *testing.T) {
	tests := []struct {
		name      string
		code      codes.Code
		innerErr  error
		wantMatch string
	}{
		{
			name:      "simple error",
			code:      codes.InvalidArgument,
			innerErr:  errors.New("bad request"),
			wantMatch: `OTLP RPC error "InvalidArgument" (3): bad request`,
		},
		{
			name:      "internal error",
			code:      codes.Internal,
			innerErr:  errors.New("server error"),
			wantMatch: `OTLP RPC error "Internal" (13): server error`,
		},
		{
			name:      "unavailable error",
			code:      codes.Unavailable,
			innerErr:  errors.New("service unavailable"),
			wantMatch: `OTLP RPC error "Unavailable" (14): service unavailable`,
		},
		{
			name:      "empty error message",
			code:      codes.Unknown,
			innerErr:  errors.New(""),
			wantMatch: `OTLP RPC error "Unknown" (2): `,
		},
		{
			name:      "nil inner error",
			code:      codes.DeadlineExceeded,
			innerErr:  nil,
			wantMatch: `OTLP RPC error "DeadlineExceeded" (4): %!s(<nil>)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ErrRPC{Code: tt.code, Err: tt.innerErr}
			got := err.Error()
			if got != tt.wantMatch {
				t.Errorf("Error() = %q, want %q", got, tt.wantMatch)
			}
		})
	}
}

func TestErrRPCIs(t *testing.T) {
	tests := []struct {
		name   string
		target ErrRPC
		err    error
		want   bool
	}{
		{
			name:   "same code",
			target: ErrRPC{Code: codes.InvalidArgument},
			err:    ErrRPC{Code: codes.InvalidArgument, Err: errors.New("test")},
			want:   true,
		},
		{
			name:   "different code",
			target: ErrRPC{Code: codes.InvalidArgument},
			err:    ErrRPC{Code: codes.Internal, Err: errors.New("test")},
			want:   false,
		},
		{
			name:   "not ErrRPC",
			target: ErrRPC{Code: codes.InvalidArgument},
			err:    errors.New("generic error"),
			want:   false,
		},
		{
			name:   "wrapped ErrRPC same code",
			target: ErrRPC{Code: codes.Internal},
			err:    fmt.Errorf("wrapped: %w", ErrRPC{Code: codes.Internal, Err: errors.New("inner")}),
			want:   true,
		},
		{
			name:   "wrapped ErrRPC different code",
			target: ErrRPC{Code: codes.Internal},
			err:    fmt.Errorf("wrapped: %w", ErrRPC{Code: codes.Unknown, Err: errors.New("inner")}),
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

func TestErrRPCAs(t *testing.T) {
	originalErr := errors.New("original error")
	rpcErr := ErrRPC{Code: codes.Internal, Err: originalErr}

	t.Run("successful assignment", func(t *testing.T) {
		var target ErrRPC
		ok := rpcErr.As(&target)
		if !ok {
			t.Fatal("As() returned false, want true")
		}
		if target.Code != codes.Internal {
			t.Errorf("target.Code = %v, want %v", target.Code, codes.Internal)
		}
		if target.Err != originalErr {
			t.Errorf("target.Err = %v, want %v", target.Err, originalErr)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		var target ErrPartial
		ok := rpcErr.As(&target)
		if ok {
			t.Error("As() returned true, want false")
		}
	})

	t.Run("not a pointer", func(t *testing.T) {
		var target ErrRPC
		ok := rpcErr.As(target)
		if ok {
			t.Error("As() returned true, want false")
		}
	})

	t.Run("nil target", func(t *testing.T) {
		ok := rpcErr.As(nil)
		if ok {
			t.Error("As() returned true, want false")
		}
	})
}

func TestErrRPCUnwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	rpcErr := ErrRPC{Code: codes.Internal, Err: innerErr}

	got := rpcErr.Unwrap()
	if got != innerErr {
		t.Errorf("Unwrap() = %v, want %v", got, innerErr)
	}

	// Test with nil inner error
	rpcErrNil := ErrRPC{Code: codes.Internal, Err: nil}
	got = rpcErrNil.Unwrap()
	if got != nil {
		t.Errorf("Unwrap() = %v, want nil", got)
	}
}

func BenchmarkErrRPC(b *testing.B) {
	innerErr := status.Error(codes.InvalidArgument, "invalid request")
	err := ErrRPC{Code: codes.InvalidArgument, Err: innerErr}

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
		target := new(ErrRPC)
		var ok bool
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ok = err.As(target)
		}
		_ = ok
	})

	b.Run("Is", func(b *testing.B) {
		var target error = ErrRPC{Code: codes.InvalidArgument}
		var ok bool
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ok = err.Is(target)
		}
		_ = ok
	})

	b.Run("Unwrap", func(b *testing.B) {
		var unwrapped error
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			unwrapped = err.Unwrap()
		}
		_ = unwrapped
	})
}
