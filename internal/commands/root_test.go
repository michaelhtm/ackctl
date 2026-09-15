// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package commands

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVersionSubcommand covers `ackctl version`; cobra wires up only the --version flag.
func TestVersionSubcommand(t *testing.T) {
	SetVersion("v1.2.3", "abc1234")
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	require.NoError(t, versionCmd.RunE(cmd, nil))
	assert.Contains(t, buf.String(), "v1.2.3")
	assert.Contains(t, buf.String(), "abc1234")
}

// TestDescribeVersion covers the `go install` path, which applies no ldflags and so would
// otherwise report the "dev" default for a tagged release.
func TestDescribeVersion(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		version, commit, module string
		want                    string
	}{
		{"ldflags win", "v0.0.1", "abc1234", "v0.0.1", "v0.0.1 (abc1234)"},
		{"go install falls back to the module version", "dev", "unknown", "v0.0.1", "v0.0.1"},
		{"local build with no module version", "dev", "unknown", "", "dev (unknown)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, describeVersion(tc.version, tc.commit, tc.module))
		})
	}
}
