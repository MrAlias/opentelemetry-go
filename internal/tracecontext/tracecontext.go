// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
Package tracecontext implements the W3C Trace Context propagator specification
(https://www.w3.org/TR/trace-context).

This package builds from the existing package:
https://github.com/iredelmeier/tracecontext.go
*/
package tracecontext

import (
	"errors"
	"fmt"
	"net/http"
	"net/textproto"
	"strings"
)

var (
	// ErrTraceContext is returned as a wrapped error when an error is
	// returned from this package.
	ErrTraceContext = errors.New("tracecontext")

	// ErrMultipleTraceParent is returne when there are multiple
	// traceparent headers present.
	ErrMultipleTraceParent = errorf("multiple traceparent headers")
)

func errorf(format string, a ...interface{}) error {
	return fmt.Errorf("%w: %s", ErrTraceContext, fmt.Sprintf(format, a...))
}

var (
	traceParentHeader = textproto.CanonicalMIMEHeaderKey("traceparent")
	traceStateHeader  = textproto.CanonicalMIMEHeaderKey("tracestate")
)

// TraceContext is the W3C Trace Context propagation format.
// It is the paired binding of the distributed trace context in the form of a
// TraceParent and TraceState.
type TraceContext struct {
	TraceParent TraceParent
	TraceState  TraceState
}

// FromHeaders parses a TraceContext from HTTP headers.
//
// The returned TraceContext's TraceParent value is valid as long as no error
// error is returned. A partial TraceParent will be returned in the
// TraceContext along with an error.
//
// It is not considered an error if the tracestate header(s) is(are) invalid.
// If the traceparent header is valid and tracestate is not, a `TraceContext`
// with an empty `TraceState` will still be returned.
func FromHeaders(headers http.Header) (TraceContext, error) {
	var tc TraceContext

	h := textproto.MIMEHeader(headers)
	if len(h[traceParentHeader]) > 1 {
		return tc, ErrMultipleTraceParent
	}

	var err error
	if tc.TraceParent, err = ParseTraceParentString(h.Get(traceParentHeader)); err != nil {
		return tc, err
	}

	var traceStates []string
	for _, traceState := range h[traceStateHeader] {
		traceStates = append(traceStates, traceState)
	}

	traceState, err := ParseTraceStateString(strings.Join(traceStates, ","))
	if err == nil {
		tc.TraceState = traceState
	}

	return tc, nil
}

// SetHeaders sets the traceparent and tracestate headers from TraceContext.
func (tc TraceContext) SetHeaders(headers http.Header) {
	headers.Set(traceParentHeader, tc.TraceParent.String())
	headers.Set(traceStateHeader, tc.TraceState.String())
}
