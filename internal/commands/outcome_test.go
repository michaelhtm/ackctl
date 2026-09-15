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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAdoptOutcome pins the exit status, since a skipped resource matched the selector but
// silently produced no CR: the manifest is incomplete in a way a log line is easy to miss.
func TestAdoptOutcome(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		resolved, skipped, matched int
		wantErr                    string
	}{
		{"all resolved", 3, 0, 3, ""},
		{"one skip is a failure", 2, 1, 3, "skipped 1 of 3"},
		{"nothing resolved names the emptier failure", 0, 3, 3, "resolved none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := adoptOutcome(tc.resolved, tc.skipped, tc.matched)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// TestAdoptOutcome_PartialSaysTheOutputIsIncomplete keeps the error actionable: the
// manifests already went to stdout, so the message has to say they are short.
func TestAdoptOutcome_PartialSaysTheOutputIsIncomplete(t *testing.T) {
	err := adoptOutcome(2, 1, 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "incomplete")
}
