// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oops // import "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc/internal/oops"

// ErrWarn is an error representing an upload where the server accepted the
// request payload but responded with a warning or suggestion for the sender.
type ErrWarn struct {
	Message string
}

var _ error = ErrWarn{}

// Error returns an error message describing the warning.
func (e ErrWarn) Error() string { return "OTLP warning: " + e.Message }

// As returns true if e can be assigned to target and makes the assignment.
// Otherwise, it returns false. This supports the errors.As() functionality.
func (e ErrWarn) As(target any) bool {
	t, ok := target.(*ErrWarn)
	if ok {
		*t = e
	}
	return ok
}

// Is returns if err is an ErrWarn error. This supports the errors.Is()
// functionality.
func (e ErrWarn) Is(err error) bool {
	_, ok := err.(ErrWarn)
	return ok
}

// ErrorType returns a string identifying the type of error.
//
// This satisfies the semconv.ErrorType interface.
func (e ErrWarn) ErrorType() string { return "Warning" }
