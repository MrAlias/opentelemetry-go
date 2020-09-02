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

package tracecontext

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"regexp"
)

const (
	// Version represents the maximum traceparent header version that is supported.
	// The library attempts optimistic forwards compatibility with higher versions.
	Version = 0
)

var (
	// ErrInvalidFormat is returned when the traceparent format is invalid.
	// Such as, if there are missing characters or a field contains an
	// unexpected character set.
	ErrInvalidFormat = errorf("invalid traceparent format")
	// ErrInvalidVersion is returned when the encoded version is invalid, i.e.
	// the version is 255.
	ErrInvalidVersion = errorf("invalid traceparent version")
	// ErrInvalidTraceID is returned when the encoded trace ID is invalid,
	// i.e. all bytes are 0
	ErrInvalidTraceID = errorf("invalid traceparent trace ID")
	// ErrInvalidSpanID is returned when the encoded span ID is invalid, i.e.
	// all bytes are 0
	ErrInvalidSpanID = errorf("invalid traceparent span ID")
)

const (
	maxVersion = 254

	numVersionBytes = 1
	numTraceIDBytes = 16
	numSpanIDBytes  = 8
	numFlagBytes    = 1
)

var (
	traceparentRe = regexp.MustCompile(`^([a-f0-9]{2})-([a-f0-9]{32})-([a-f0-9]{16})-([a-f0-9]{2})(-.*)?$`)

	invalidTraceIDAllZeroes = make([]byte, numTraceIDBytes, numTraceIDBytes)
	invalidSpanIDAllZeroes  = make([]byte, numSpanIDBytes, numSpanIDBytes)
)

// TraceParent is the W3C Trace Context traceparent header field value.
// It includes information about a span and its part in a trace.
type TraceParent struct {
	// Version is the version of the W3C Trace Context specification this
	// TraceParent implements.
	Version uint8
	// TraceID is the trace identifier of the trace a span is a part of.
	TraceID [16]byte
	// SpanID is the span identifier of the span that sent or is sending this
	// TraceParent.
	SpanID [8]byte
	// Flags are the tracing flags describing recommendations given by the
	// caller.
	Flags Flags
}

// String encodes the TraceParent as a string conforming to the W3C Trace
// Context specification. No validation of the TraceParent is performed. If
// the TraceParent contains invalid fields, i.e. a TraceID with only 0 bytes,
// the returned string will also be invalid.
func (tp TraceParent) String() string {
	return fmt.Sprintf("%02x-%032x-%016x-%s", tp.Version, tp.TraceID, tp.SpanID, tp.Flags)
}

// Flags are the recommendations provided by a sender of how a trace should be
// interpreted. These flags need to be understood in context of who the sender
// was and what level of trust they are attributed.
type Flags struct {
	// Sampled denotes that the caller may have recorded trace data. When this
	// value is false, the caller did not record trace data out-of-band.
	Sampled bool
}

// String encodes the Flags in an 8-bit encoding returned as a string.
func (f Flags) String() string {
	var flags [1]byte
	if f.Sampled {
		flags[0] = 1
	}
	return fmt.Sprintf("%02x", flags)
}

// ParseTraceParent decodes a TraceParent from a byte array containing a valid
// traceparent header field value. If the traceparent header field value is
// invalid an error is returned.
func ParseTraceParent(b []byte) (TraceParent, error) {
	return parseTraceParent(b)
}

// ParseTraceParentString decodes a TraceParent from a string containing a
// valid traceparent header field value. If the traceparent header field value
// is invalid an error is returned.
func ParseTraceParentString(s string) (TraceParent, error) {
	return parseTraceParent([]byte(s))
}

func parseTraceParent(b []byte) (tp TraceParent, err error) {
	matches := traceparentRe.FindSubmatch(b)
	if len(matches) < 6 {
		err = fmt.Errorf("%w: %v", ErrInvalidFormat, b)
		return
	}

	var version uint8
	if version, err = parseVersion(matches[1]); err != nil {
		return
	}
	if version == Version && len(matches[5]) > 0 {
		err = fmt.Errorf("%w: %v", ErrInvalidFormat, b)
		return
	}

	var traceID [16]byte
	if traceID, err = parseTraceID(matches[2]); err != nil {
		return
	}

	var spanID [8]byte
	if spanID, err = parseSpanID(matches[3]); err != nil {
		return
	}

	var flags Flags
	if flags, err = parseFlags(matches[4]); err != nil {
		return
	}

	tp.Version = Version
	tp.TraceID = traceID
	tp.SpanID = spanID
	tp.Flags = flags

	return tp, nil
}

func parseVersion(b []byte) (uint8, error) {
	version, ok := parseEncodedSegment(b, numVersionBytes)
	if !ok {
		return 0, fmt.Errorf("%w: %v", ErrInvalidFormat, b)
	}
	if version[0] > maxVersion {
		return 0, fmt.Errorf("%w: %v", ErrInvalidVersion, b)
	}
	return version[0], nil
}

func parseTraceID(b []byte) (traceID [16]byte, err error) {
	id, ok := parseEncodedSegment(b, numTraceIDBytes)
	if !ok {
		return traceID, fmt.Errorf("%w: %v", ErrInvalidFormat, b)
	}
	if bytes.Equal(id, invalidTraceIDAllZeroes) {
		return traceID, fmt.Errorf("%w: %v", ErrInvalidTraceID, b)
	}

	copy(traceID[:], id)

	return traceID, nil
}

func parseSpanID(b []byte) (spanID [8]byte, err error) {
	id, ok := parseEncodedSegment(b, numSpanIDBytes)
	if !ok {
		return spanID, fmt.Errorf("%w: %v", ErrInvalidFormat, b)
	}
	if bytes.Equal(id, invalidSpanIDAllZeroes) {
		return spanID, fmt.Errorf("%w: %v", ErrInvalidSpanID, b)
	}

	copy(spanID[:], id)

	return spanID, nil
}

func parseFlags(b []byte) (Flags, error) {
	flags, ok := parseEncodedSegment(b, numFlagBytes)
	if !ok {
		return Flags{}, fmt.Errorf("%w: %v", ErrInvalidFormat, b)
	}

	return Flags{
		Sampled: (flags[0] & 1) == 1,
	}, nil
}

func parseEncodedSegment(src []byte, expectedLen int) ([]byte, bool) {
	dst := make([]byte, hex.DecodedLen(len(src)))
	if n, err := hex.Decode(dst, src); n != expectedLen || err != nil {
		return dst, false
	}
	return dst, true
}
