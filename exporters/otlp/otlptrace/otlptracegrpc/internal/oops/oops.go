// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package oops provides errors for OpenTelemetry OTLP Problematic Scenarios.
package oops // import "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc/internal/oops"

import (
	"errors"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ParseError parses the error and partial success details from an
// [coltracepb.ExportTraceServiceResponse] returned by a gRPC Export call. It
// returns a single error representing the overall result of the call.
//
// If err is non-nil and has a gRPC status code other than [codes.OK], it is
// wrapped with an [ErrRPC]. If err is non-nil but has a gRPC status code of
// [codes.OK], it is ignored as this may represent a partial success.
//
// If resp contains partial success details, they are parsed and combined with
// err. If there are rejected spans, an [ErrPartial] is included. If there is
// an error message but no rejected spans, an [ErrWarn] is included.
//
// The returned error may be nil (indicating full success), an [ErrRPC], an
// [ErrPartial], an [ErrWarn], or a combination of these using [errors.Join].
func ParseError(resp *coltracepb.ExportTraceServiceResponse, err error) error {
	if err != nil {
		c := status.Code(err)
		if c != codes.OK {
			// Use a more OTel friendly error type.
			err = ErrRPC{Code: c, Err: err}
		} else {
			// Handle the case where err is non-nil but the code is OK. This
			// may happen when a partial success is reported.
			err = nil
		}
	}

	// If there are partial success details, parse them.
	ps := resp.GetPartialSuccess()
	n := ps.GetRejectedSpans()
	msg := ps.GetErrorMessage()
	if n != 0 {
		err = errors.Join(err, ErrPartial{Rejected: n, Message: msg})
	} else if msg != "" {
		err = errors.Join(err, ErrWarn{Message: msg})
	}
	return err
}
