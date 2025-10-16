// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oops

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestParseError(t *testing.T) {
	errRPCInvalidArg := status.Error(codes.InvalidArgument, "invalid argument")
	errRPCUnavailable := status.Error(codes.Unavailable, "service unavailable")
	errRPCInternal := status.Error(codes.Internal, "internal error")

	tests := []struct {
		name string
		resp *coltracepb.ExportTraceServiceResponse
		err  error
		want error
	}{
		{
			name: "NilErrorNilResponse",
		},
		{
			name: "NilErrorEmptyResponse",
			resp: &coltracepb.ExportTraceServiceResponse{},
		},
		{
			name: "OKStatusError",
			err:  status.Error(codes.OK, "success"),
		},
		{
			name: "InvalidArgumentError",
			err:  errRPCInvalidArg,
			want: &ErrRPC{
				Code: codes.InvalidArgument,
				Err:  errRPCInvalidArg,
			},
		},
		{
			name: "PartialSuccessResponse",
			resp: &coltracepb.ExportTraceServiceResponse{
				PartialSuccess: &coltracepb.ExportTracePartialSuccess{
					RejectedSpans: 5,
					ErrorMessage:  "some spans were rejected",
				},
			},
			want: ErrPartial{Rejected: 5, Message: "some spans were rejected"},
		},
		{
			name: "WarnResponse",
			resp: &coltracepb.ExportTraceServiceResponse{
				PartialSuccess: &coltracepb.ExportTracePartialSuccess{
					ErrorMessage: "deprecated field used",
				},
			},
			want: ErrWarn{Message: "deprecated field used"},
		},
		{
			name: "ErrorWithPartialSuccess",
			resp: &coltracepb.ExportTraceServiceResponse{
				PartialSuccess: &coltracepb.ExportTracePartialSuccess{
					RejectedSpans: 3,
					ErrorMessage:  "connection issues",
				},
			},
			err: errRPCUnavailable,
			want: errors.Join(
				ErrRPC{Code: codes.Unavailable, Err: errRPCUnavailable},
				ErrPartial{Rejected: 3, Message: "connection issues"},
			),
		},
		{
			name: "ErrorOKWithPartialSuccess",
			resp: &coltracepb.ExportTraceServiceResponse{
				PartialSuccess: &coltracepb.ExportTracePartialSuccess{
					RejectedSpans: 2,
					ErrorMessage:  "some validation errors",
				},
			},
			err:  status.Error(codes.OK, "partial success"),
			want: ErrPartial{Rejected: 2, Message: "some validation errors"},
		},
		{
			name: "gRPC error with warning only",
			resp: &coltracepb.ExportTraceServiceResponse{
				PartialSuccess: &coltracepb.ExportTracePartialSuccess{
					RejectedSpans: 0,
					ErrorMessage:  "use newer API version",
				},
			},
			err: errRPCInternal,
			want: errors.Join(
				ErrRPC{Code: codes.Internal, Err: errRPCInternal},
				ErrWarn{Message: "use newer API version"},
			),
		},
		{
			name: "EmptyPartialSuccessMessage",
			resp: &coltracepb.ExportTraceServiceResponse{
				PartialSuccess: &coltracepb.ExportTracePartialSuccess{
					RejectedSpans: 10,
				},
			},
			want: ErrPartial{Rejected: 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ParseError(tt.resp, tt.err))
		})
	}
}

func TestParseErrorEdgeCases(t *testing.T) {
	t.Run("nil response", func(t *testing.T) {
		err := ParseError(nil, nil)
		if err != nil {
			t.Errorf("ParseError(nil, nil) = %v, want nil", err)
		}

		// Test that we don't panic and handle nil properly
		grpcErr := status.Error(codes.Internal, "test error")
		err = ParseError(nil, grpcErr)
		var rpcErr ErrRPC
		if !errors.As(err, &rpcErr) {
			t.Error("Expected ErrRPC in result")
		}
	})

	t.Run("nil response with gRPC error", func(t *testing.T) {
		grpcErr := status.Error(codes.Internal, "internal error")
		err := ParseError(nil, grpcErr)

		var rpcErr ErrRPC
		if !errors.As(err, &rpcErr) {
			t.Error("Expected ErrRPC in result")
		}
		if rpcErr.Code != codes.Internal {
			t.Errorf("ErrRPC.Code = %v, want %v", rpcErr.Code, codes.Internal)
		}
	})

	t.Run("generic error (not gRPC status)", func(t *testing.T) {
		genericErr := errors.New("generic error")
		resp := &coltracepb.ExportTraceServiceResponse{}

		result := ParseError(resp, genericErr)

		// A generic error should still be wrapped as ErrRPC with Unknown code
		var rpcErr ErrRPC
		if !errors.As(result, &rpcErr) {
			t.Error("Expected generic error to be wrapped as ErrRPC")
		}
		if rpcErr.Code != codes.Unknown {
			t.Errorf("ErrRPC.Code = %v, want %v", rpcErr.Code, codes.Unknown)
		}
	})

	t.Run("large rejected spans count", func(t *testing.T) {
		resp := &coltracepb.ExportTraceServiceResponse{
			PartialSuccess: &coltracepb.ExportTracePartialSuccess{
				RejectedSpans: 1000000,
				ErrorMessage:  "too many spans",
			},
		}

		err := ParseError(resp, nil)
		var partialErr ErrPartial
		if !errors.As(err, &partialErr) {
			t.Fatal("Expected ErrPartial in result")
		}
		if partialErr.Rejected != 1000000 {
			t.Errorf("ErrPartial.Rejected = %d, want 1000000", partialErr.Rejected)
		}
	})

	t.Run("complex scenario with all possible combinations", func(t *testing.T) {
		// Test complex scenario where GetPartialSuccess returns nil (edge case)
		resp := &coltracepb.ExportTraceServiceResponse{
			PartialSuccess: nil,
		}
		grpcErr := status.Error(codes.Unauthenticated, "auth failed")

		err := ParseError(resp, grpcErr)
		var rpcErr ErrRPC
		if !errors.As(err, &rpcErr) {
			t.Error("Expected ErrRPC in result")
		}
		if rpcErr.Code != codes.Unauthenticated {
			t.Errorf("ErrRPC.Code = %v, want %v", rpcErr.Code, codes.Unauthenticated)
		}
	})
}

func BenchmarkParseError(b *testing.B) {
	resp := &coltracepb.ExportTraceServiceResponse{
		PartialSuccess: &coltracepb.ExportTracePartialSuccess{
			RejectedSpans: 5,
			ErrorMessage:  "some spans rejected",
		},
	}
	grpcErr := status.Error(codes.InvalidArgument, "bad request")

	b.Run("NoError", func(b *testing.B) {
		emptyResp := &coltracepb.ExportTraceServiceResponse{}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ParseError(emptyResp, nil)
		}
	})

	b.Run("RPCErrorOnly", func(b *testing.B) {
		emptyResp := &coltracepb.ExportTraceServiceResponse{}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ParseError(emptyResp, grpcErr)
		}
	})

	b.Run("PartialErrorOnly", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ParseError(resp, nil)
		}
	})

	b.Run("CombinedErrors", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ParseError(resp, grpcErr)
		}
	})
}
