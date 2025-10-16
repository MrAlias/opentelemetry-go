// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oops // import "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc/internal/oops"

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
)

// ErrRPC is an error representing a gRPC failure: a non-OK gRPC status code.
//
// The Err field contains the original error returned by gRPC. The Code field
// contains the gRPC status code.
//
// This error type is used to represent errors returned by gRPC calls. It is
// distinct from ErrPartial, which represents a partial success response from
// the server.
type ErrRPC struct {
	Code codes.Code
	Err  error
}

var _ error = ErrRPC{}

// ErrorType returns a string identifying the type of error: the gRPC status
// code string.
//
// This satisfies the semconv.ErrorType interface.
func (e ErrRPC) ErrorType() string { return e.Code.String() }

// Error returns an error message describing the gRPC failure.
func (e ErrRPC) Error() string {
	// TODO: optimize.
	return fmt.Sprintf("OTLP RPC error %[1]q (%[1]d): %[2]s", e.Code, e.Err)
}

// Is returns if err is an ErrRPC error with the same Code as e. This supports
// the errors.Is() functionality.
func (e ErrRPC) Is(err error) bool {
	var target ErrRPC
	if errors.As(err, &target) {
		return e.Code == target.Code
	}
	return false
}

// As returns true if e can be assigned to target and makes the assignment.
// Otherwise, it returns false. This supports the errors.As() functionality.
func (e ErrRPC) As(target any) bool {
	t, ok := target.(*ErrRPC)
	if ok {
		*t = e
	}
	return ok
}

// Unwrap returns the underlying error. This supports the errors.Unwrap()
// functionality.
func (e ErrRPC) Unwrap() error { return e.Err }
