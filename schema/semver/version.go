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

// Package semver provides limited functionality for [Semantic Versions] in Go.
//
// This package is intended to be used in the special situation of
// OpenTelemetry schema versions. These versions should only need to support
// major, minor, and patch parts of semantic versions.
//
// This package is based on the more complete functionality provided by
// github.com/github.com/Masterminds/semver/v3. It does not support pre-release
// or metadata parts of versions and is missing many helper functions. That
// package should be prefered for most situations.
//
// [Semantic Versions]: http://semver.org
package semver // import "go.opentelemetry.io/otel/schema/semver/semver"

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Version represents the major, minor, and patch parts of a single semantic
// version.
type Version struct {
	Major, Minor, Patch uint64
}

// NewVersion parses the version string str into a SemVer if it is valid and
// supported, otherwise an error is returned. Only complete versions with the
// major, minor, and patch parts are supported (i.e. both "1" and "1.0" are not
// supported). An error is returned if these parts are not all defined in str.
//
// The pre-release and metadata parts of a semantic version are not supported
// by NewVersion (similar to how they are not supported by SemVer). An error is
// returned if these parts are defined in str.
func NewVersion(str string) (Version, error) {
	if len(str) == 0 {
		return Version{}, errors.New("empty version string")
	}

	parts := strings.SplitN(str, ".", 3)
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("invalid semantic version: %s", str)
	}

	if strings.ContainsAny(parts[2], "-+") {
		return Version{}, fmt.Errorf("unsupported version: %s", str)
	}

	const nums = "0123456789"
	for _, p := range parts {
		nonNumIdx := strings.IndexFunc(p, func(r rune) bool {
			return !strings.ContainsRune(nums, r)
		})
		if nonNumIdx != -1 {
			return Version{}, fmt.Errorf("invalid version characters %q: %s", p, str)
		}

		if len(p) > 1 && p[0] == '0' {
			return Version{}, fmt.Errorf("version segment starts with 0: %s", str)
		}
	}

	var (
		err error
		s   Version
	)

	s.Major, err = strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return Version{}, err
	}
	s.Minor, err = strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return Version{}, err
	}
	s.Patch, err = strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return Version{}, err
	}

	return s, nil
}

// String returns v as a string.
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare compares this SemVer to another one. It returns -1, 0, or 1 if the
// version smaller, equal, or larger than the other version.
func (v Version) Compare(o Version) int {
	cmp := func(a, b uint64) int {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	}

	if d := cmp(v.Major, o.Major); d != 0 {
		return d
	}
	if d := cmp(v.Minor, o.Minor); d != 0 {
		return d
	}
	if d := cmp(v.Patch, o.Patch); d != 0 {
		return d
	}
	return 0
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (v *Version) UnmarshalText(text []byte) error {
	temp, err := NewVersion(string(text))
	if err != nil {
		return err
	}

	*v = temp

	return nil
}

// MarshalText implements the encoding.TextMarshaler interface.
func (v Version) MarshalText() ([]byte, error) {
	return []byte(v.String()), nil
}
