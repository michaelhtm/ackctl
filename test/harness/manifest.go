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

//go:build integration || e2e

package harness

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// Manifest mirrors the emitted CR. It is declared here, not imported, so the assertions do
// not share a definition with the code that produced the output.
type Manifest struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name        string            `json:"name"`
		Namespace   string            `json:"namespace"`
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata"`
	Spec map[string]interface{} `json:"spec"`
}

// docSeparator splits a YAML stream on its document markers. Annotation values hold
// JSON on a single line, so no value can contain a line that is bare "---".
var docSeparator = regexp.MustCompile(`(?m)^---$`)

// DNS1123Subdomain is the name format the Kubernetes API server enforces. Checking it here
// catches a bad name before apply time.
var DNS1123Subdomain = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)

// CleanupErr reports a teardown failure without failing the test, which would mask the
// result the test measured.
func CleanupErr(t *testing.T, err error, what string) {
	t.Helper()
	if err != nil {
		fmt.Fprintf(os.Stderr, "LEAKED: could not delete %s: %v\n", what, err)
		t.Logf("LEAKED %s: %v", what, err)
	}
}

func ParseManifests(t *testing.T, stdout string) []Manifest {
	t.Helper()
	var out []Manifest
	for _, doc := range docSeparator.Split(stdout, -1) {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		var cr Manifest
		require.NoError(t, yaml.Unmarshal([]byte(doc), &cr),
			"emitted document is not valid YAML:\n%s", doc)
		out = append(out, cr)
	}
	return out
}
