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

//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/aws-controllers-k8s/ackctl/test/fixtures"
	"github.com/aws-controllers-k8s/ackctl/test/harness"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws-controllers-k8s/ackctl/metadata"
)

// TestEveryCatalogKindHasAnEntry fails for a newly adoptable kind until someone writes a
// fixture for it or records why it cannot have one.
func TestEveryCatalogKindHasAnEntry(t *testing.T) {
	catalog, err := metadata.Load()
	require.NoError(t, err)

	entries := map[string]fixtures.KindEntry{}
	for _, e := range fixtures.Kinds {
		key := e.Service + "/" + e.Kind
		_, dup := entries[key]
		require.False(t, dup, "duplicate registry entry for %s", key)
		entries[key] = e
	}

	var uncovered []string
	for _, r := range catalog.Resources() {
		key := r.Service + "/" + r.Kind
		if _, ok := entries[key]; !ok {
			uncovered = append(uncovered, key)
		}
		delete(entries, key)
	}
	sort.Strings(uncovered)
	assert.Empty(t, uncovered,
		"these adoptable kinds have no e2e entry: add a fixture to the registry, or a skip with a reason")

	var stale []string
	for key := range entries {
		stale = append(stale, key)
	}
	sort.Strings(stale)
	assert.Empty(t, stale, "these registry entries name no adoptable kind and are dead")

	for _, e := range fixtures.Kinds {
		if e.Create == nil {
			assert.NotEmpty(t, e.Skip, "%s/%s has neither a fixture nor a skip reason", e.Service, e.Kind)
			assert.NotEmpty(t, e.Why, "%s/%s is skipped without saying why", e.Service, e.Kind)
			continue
		}
		assert.Empty(t, e.Skip, "%s/%s has both a fixture and a skip reason", e.Service, e.Kind)
	}

	t.Logf("coverage: %d of %d adoptable kinds have a fixture", countFixtures(), len(catalog.Resources()))
	for _, r := range []fixtures.SkipReason{fixtures.SkipCost, fixtures.SkipPrereq, fixtures.SkipSlow, fixtures.SkipRegion} {
		t.Logf("  skipped (%s): %d", r, countSkipped(r))
	}
}

func countFixtures() int {
	n := 0
	for _, e := range fixtures.Kinds {
		if e.Create != nil {
			n++
		}
	}
	return n
}

func countSkipped(r fixtures.SkipReason) int {
	n := 0
	for _, e := range fixtures.Kinds {
		if e.Skip == r {
			n++
		}
	}
	return n
}

// selectedKinds narrows the run to ACK_TEST_SERVICES when set, so a run limited to one
// service skips the fixtures it does not need.
func selectedKinds(t *testing.T) []fixtures.KindEntry {
	var out []fixtures.KindEntry
	want := os.Getenv("ACK_TEST_SERVICES")
	allowed := map[string]bool{}
	for _, s := range strings.Split(want, ",") {
		if s = strings.TrimSpace(strings.ToLower(s)); s != "" {
			allowed[s] = true
		}
	}
	for _, e := range fixtures.Kinds {
		if e.Create == nil {
			continue
		}
		if len(allowed) > 0 && !allowed[strings.ToLower(e.Service)] {
			continue
		}
		out = append(out, e)
	}
	require.NotEmpty(t, out, "ACK_TEST_SERVICES=%q selected no fixtures", want)
	return out
}

// TestAdoptEveryKind asserts only on the YAML adopt emits. No cluster and no controller are
// involved: the real AWS resource exists so that AWS itself, rather than a hand-written
// fixture, supplies the ARN and the adoption-fields to compare against.
//
// Every fixture is created before anything is asserted, because the wait for the Tagging API
// to index them dominates the runtime.
func TestAdoptEveryKind(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()
	h := harness.New(ctx, t)
	f := fixtures.New(h)
	selected := selectedKinds(t)

	want := map[string]string{}
	fields := map[string]map[string]string{}
	arns := map[string]string{}
	for _, e := range selected {
		label := e.Service + "/" + e.Kind
		arnStr, wantFields := e.Create(ctx, t, f)
		t.Logf("created %-42s %s", label, arnStr)
		want[arnStr] = label
		fields[label] = wantFields
		arns[label] = arnStr
	}

	h.WaitForTagIndex(ctx, t, want)

	for _, e := range selected {
		label := e.Service + "/" + e.Kind
		t.Run(label, func(t *testing.T) {
			stdout, stderr := h.Adopt(t, e.Service, e.Kind)
			assertAdoptedOne(t, h, e, arns[label], stdout, stderr, fields[label])
		})
	}
}

// assertAdoptedOne checks one kind end to end: a single manifest, the right GVK, the
// annotations, and adoption-fields matching what the AWS API reported.
func assertAdoptedOne(
	t *testing.T,
	h *harness.Harness,
	e fixtures.KindEntry,
	arnStr string,
	stdout, stderr string,
	wantFields map[string]string,
) {
	t.Helper()

	catalog, err := metadata.Load()
	require.NoError(t, err)
	res, ok := catalog.LookupByServiceKind(e.Service, e.Kind)
	require.True(t, ok)

	crs := harness.ParseManifests(t, stdout)
	require.Len(t, crs, 1,
		"the run tag is unique to this test, so exactly one resource should have matched\nstderr:\n%s", stderr)
	cr := crs[0]

	assert.Equal(t, res.GroupVersion(), cr.APIVersion)
	assert.Equal(t, e.Kind, cr.Kind)
	assert.Empty(t, cr.Spec, "adoption manifests carry no spec; the controller fills it in")

	ann := cr.Metadata.Annotations
	assert.Equal(t, "adopt", ann["services.k8s.aws/adoption-policy"])
	assert.Equal(t, "true", ann["services.k8s.aws/read-only"], "the default flags must be observe-only")
	assert.Equal(t, "retain", ann["services.k8s.aws/deletion-policy"])
	assert.Equal(t, h.Region, ann["services.k8s.aws/region"])

	var got map[string]string
	require.NoError(t, json.Unmarshal([]byte(ann["services.k8s.aws/adoption-fields"]), &got),
		"adoption-fields must be a JSON object; ACK parses it")
	assert.Equal(t, wantFields, got,
		"adoption-fields do not match what AWS reported for %s, so the controller would look up "+
			"the wrong resource or fail to find it", arnStr)

	assert.Regexp(t, harness.DNS1123Subdomain, cr.Metadata.Name, "the API server rejects names outside this format")
	assert.LessOrEqual(t, len(cr.Metadata.Name), 253)
	assert.NotEmpty(t, cr.Metadata.Labels["ack.k8s.aws/adoption-set"])

	assert.NotContains(t, stdout, "resolved ", "the summary leaked into stdout and would corrupt the YAML")
	assert.Contains(t, stderr, "resolved 1")
}
