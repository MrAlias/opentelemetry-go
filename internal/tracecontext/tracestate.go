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
	"fmt"
	"regexp"
	"strings"
)

var (
	// ErrInvalidListMember is returned when at least one list member is
	// invalid, i.e. the list member contains an unexpected character.
	ErrInvalidListMember = errorf("invalid tracestate list member")
	// ErrDuplicateListMemberKey is returned when at two or more list members
	// contain the same vendor-tenant pair.
	ErrDuplicateListMemberKey = errorf("duplicate list member key in tracestate")
	// ErrTooManyListMembers is returned when the list contains more than the
	// maximum number of members per the spec (currently 32:
	// https://www.w3.org/TR/trace-context/#tracestate-header-field-values) .
	ErrTooManyListMembers = errorf("too many list members in tracestate")
)

const (
	maxMembers = 32

	delimiter = ","
)

var (
	tracestateRe = regexp.MustCompile(`^\s*(?:([a-z0-9_\-*/]{1,241})@([a-z0-9_\-*/]{1,14})|([a-z0-9_\-*/]{1,256}))=([\x20-\x2b\x2d-\x3c\x3e-\x7e]*[\x21-\x2b\x2d-\x3c\x3e-\x7e])\s*$`)
)

// TraceState is the W3C Trace Context tracestate header field value.
// It is a list of Members that should be propagated to new spans started in a
// trace.
type TraceState []Member

// String encodes all Members of the TraceState into a single string
// conforming to the W3C Trace Context specification. No validation of the
// TraceState is performed. If the TraceState contains invalid Members, the
// returned string will also contain these invalid entries.
func (ts TraceState) String() string {
	var members []string
	for _, member := range ts {
		members = append(members, member.String())
	}
	return strings.Join(members, ",")
}

// Member contains vendor-specific data that should be propagated across all
// new spans started within a trace.
type Member struct {
	// Vendor is a key representing a particular trace vendor.
	Vendor string
	// Tenant is a key used to distinguish between tenants of a multi-tenant
	// trace vendor.
	Tenant string
	// Value is the particular data that the vendor intents to pass to child
	// spans.
	Value string
}

// String encodes a Member as a string conforming to the W3C Trace Context
// specification. No validation of the Member is performed. If the Member
// contains invalid fields, i.e. a vendor contains invalid characters, the
// returned string will also include these invalid values.
func (m Member) String() string {
	if m.Tenant == "" {
		return fmt.Sprintf("%s=%s", m.Vendor, m.Value)
	}
	return fmt.Sprintf("%s@%s=%s", m.Vendor, m.Tenant, m.Value)
}

// ParseTraceState decodes a `TraceState` from a byte array containing a valid
// tracestate header field value. If the tracestate header field value is
// invalid an error is returned.
func ParseTraceState(traceState []byte) (TraceState, error) {
	return parseTraceState(string(traceState))
}

// ParseTraceStateString decodes a `TraceState` from a string containing a
// valid tracestate header field value. If the tracestate header field value
// is invalid an error is returned.
func ParseTraceStateString(traceState string) (TraceState, error) {
	return parseTraceState(traceState)
}

func parseTraceState(traceState string) (ts TraceState, err error) {
	found := make(map[string]interface{})

	members := strings.Split(traceState, delimiter)

	for _, member := range members {
		if len(member) == 0 {
			continue
		}

		var m Member
		m, err = parseMember(member)
		if err != nil {
			return
		}

		key := fmt.Sprintf("%s%s", m.Vendor, m.Tenant)
		if _, ok := found[key]; ok {
			err = ErrDuplicateListMemberKey
			return
		}
		found[key] = nil

		ts = append(ts, m)

		if len(ts) > maxMembers {
			err = ErrTooManyListMembers
			return
		}
	}

	return
}

func parseMember(s string) (Member, error) {
	matches := tracestateRe.FindStringSubmatch(s)
	if len(matches) != 5 {
		return Member{}, ErrInvalidListMember
	}

	vendor := matches[1]
	if vendor == "" {
		vendor = matches[3]
	}

	return Member{
		Vendor: vendor,
		Tenant: matches[2],
		Value:  matches[4],
	}, nil
}
