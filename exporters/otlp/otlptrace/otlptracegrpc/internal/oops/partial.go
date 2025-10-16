// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oops // import "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc/internal/oops"

import (
	"bytes"
	"strconv"
	"sync"
)

// ErrPartial is an error representing a partial upload failure.
//
// This error is used when an OTLP trace export request partially
// succeeds. It contains the number of spans that were rejected by the server
// and an optional message with more details about the failure.
type ErrPartial struct {
	Rejected int64
	Message  string
}

var _ error = ErrPartial{}

const bufPoolMaxCap = 128

// bufPool is a pool of reusable bytes.Buffers to reduce memory allocations
// when building error messages.
var bufPool = &sync.Pool{New: func() any { return new(bytes.Buffer) }}

func getBuffer() *bytes.Buffer {
	return bufPool.Get().(*bytes.Buffer)
}

func putBuffer(b *bytes.Buffer) {
	// Proper usage of a sync.Pool requires each entry to have
	// approximately the same memory cost. To obtain this property when the
	// stored type contains a variably-sized buffer, we add a hard limit on
	// the maximum buffer to place back in the pool. If the buffer is
	// larger than the limit, we drop the buffer.
	//
	// See https://golang.org/issue/23199.
	if b.Cap() > bufPoolMaxCap {
		return
	}
	b.Reset()
	bufPool.Put(b)
}

// Error returns an error message describing the partial upload failure.
func (e ErrPartial) Error() string {
	b := getBuffer()
	defer putBuffer(b)

	// Construct the error message manually to avoid memory allocations in
	// strconv, fmt, and strings packages.
	//
	// Format: "OTLP partial upload success: <msg>: <num> spans rejected"
	//
	//   - "OTLP partial upload success: " is 29 bytes.
	//   - <msg> is len(e.Message) bytes.
	//   - ": " is 2 bytes.
	//   - <num> is at most 19 bytes (int64)[^1].
	//   - " spans rejected" is 15 bytes.
	//
	// So the total length is 29 + len(e.Message) + 2 + 19 + 15 bytes
	// (i.e. 65 + len(e.Message)).
	//
	// [^1]: Technically, the maximum length of an int64 in base 10 is 20
	// ("-9223372036854775808"), but the negative minimum value of an int64 is
	// not a valid value for the number of rejected spans.
	const n = 29 + 2 + 19 + 15
	b.Grow(n + len(e.Message))

	b.WriteString("OTLP partial upload success: ")

	if e.Message != "" {
		b.WriteString(e.Message)
		b.WriteString(": ")
	}

	buf := b.Bytes()
	const base = 10
	buf = strconv.AppendInt(buf, e.Rejected, base)
	b.Reset()
	b.Write(buf)

	b.WriteString(" spans rejected")
	return b.String()
}

// As returns true if e can be assigned to target and makes the assignment.
// Otherwise, it returns false. This supports the errors.As() functionality.
func (e ErrPartial) As(target any) bool {
	t, ok := target.(*ErrPartial)
	if ok {
		*t = e
	}
	return ok
}

// Is returns if err is an ErrPartial error. This supports the errors.Is()
// functionality.
func (e ErrPartial) Is(err error) bool {
	_, ok := err.(ErrPartial)
	return ok
}

// ErrorType returns a string identifying the type of error.
//
// This satisfies the semconv.ErrorType interface.
func (e ErrPartial) ErrorType() string { return "PartialSuccess" }
