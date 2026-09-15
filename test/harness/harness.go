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

// Package harness drives the built ackctl binary against real AWS. It is shared by the
// integration and e2e suites, and CREATES AND DELETES real resources, all free of charge and
// tagged with a run-unique value.
package harness

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	rgt "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	rgttypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/require"
)

// RunTagKey is the tag every fixture carries, with a value unique per run so the
// selector cannot match anything this run did not create.
const RunTagKey = "ack-integration-run"

// ResourcePrefix marks created resources as test scaffolding by name as well as by
// tag, so anything left behind by a killed run is recognisable.
const ResourcePrefix = "ack-it-"

// SharedTagKey tags the prerequisites several fixtures share. It is not RunTagKey, so a
// shared VPC cannot match the selector of the run adopting ec2/VPC.
const SharedTagKey = "ack-integration-shared"

// TagIndexTimeout bounds the wait for a freshly tagged resource to become visible
// to GetResources, which is an eventually consistent index.
const TagIndexTimeout = 6 * time.Minute

type Harness struct {
	Cfg       aws.Config
	Region    string
	Partition string
	Account   string
	bin       string
	RunID     string
	rgt       *rgt.Client
}

func New(ctx context.Context, t *testing.T) *Harness {
	t.Helper()

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	require.NoError(t, err, "this test needs AWS credentials")
	require.NotEmpty(t, cfg.Region,
		"no region resolved: set AWS_REGION. GetResources is regional, and the region "+
			"is written to every emitted CR")

	ident, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	require.NoError(t, err, "could not identify the calling account")
	callerARN, err := arn.Parse(aws.ToString(ident.Arn))
	require.NoError(t, err)

	h := &Harness{
		Cfg:       cfg,
		Region:    cfg.Region,
		Partition: callerARN.Partition,
		Account:   aws.ToString(ident.Account),
		RunID:     randomID(t),
		rgt:       rgt.NewFromConfig(cfg),
	}
	h.bin = buildBinary(t)

	t.Logf("region %s, run %s=%s", h.Region, RunTagKey, h.RunID)
	return h
}

// randomID is random, not clock-derived, so two runs starting in the same second cannot
// share a tag value and tear down each other's resources.
func randomID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 6)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return hex.EncodeToString(b)
}

// buildBinary compiles the real CLI, so flag parsing, region resolution and the stdout/stderr
// split stay in scope.
func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "ackctl")
	cmd := exec.Command("go", "build", "-o", bin, "../../cmd/ackctl")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "building ackctl failed:\n%s", out)
	return bin
}

func (h *Harness) ResourceName() string { return ResourcePrefix + h.RunID }

func (h *Harness) RunTag() string { return RunTagKey + "=" + h.RunID }

// Adopt runs `ackctl adopt` for one kind and returns stdout and stderr separately.
func (h *Harness) Adopt(t *testing.T, service, kind string, extra ...string) (string, string) {
	t.Helper()
	args := append([]string{
		"adopt",
		"--service", service,
		"--kind", kind,
		"--tag", h.RunTag(),
		"--region", h.Region,
	}, extra...)

	var stdout, stderr strings.Builder
	cmd := exec.Command(h.bin, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	require.NoError(t, err, "ackctl %s failed\nstdout:\n%s\nstderr:\n%s",
		strings.Join(args, " "), stdout.String(), stderr.String())
	return stdout.String(), stderr.String()
}

// WaitForTagIndex blocks until every ARN is visible to GetResources under this
// run's tag. A timeout is a real finding: it means a kind the catalog calls
// adoptable is not returned by the Tagging API at all.
func (h *Harness) WaitForTagIndex(ctx context.Context, t *testing.T, want map[string]string) {
	t.Helper()
	deadline := time.Now().Add(TagIndexTimeout)
	pending := map[string]string{}
	for a, label := range want {
		pending[a] = label
	}

	for attempt := 1; ; attempt++ {
		found, err := h.indexedARNs(ctx)
		require.NoError(t, err, "querying the Tagging API failed")
		for a := range pending {
			if found[a] {
				delete(pending, a)
			}
		}
		if len(pending) == 0 {
			t.Logf("all %d resource(s) indexed after %d poll(s)", len(want), attempt)
			return
		}
		if time.Now().After(deadline) {
			var missing []string
			for a, label := range pending {
				missing = append(missing, fmt.Sprintf("%s (%s)", label, a))
			}
			t.Fatalf("after %s, %d resource(s) never appeared in the Tagging API:\n  %s\n"+
				"They exist and are tagged, so either indexing is unusually slow, or the "+
				"Tagging API does not index these kinds at all — in which case the catalog "+
				"should not offer them for tag-based adoption.",
				TagIndexTimeout, len(pending), strings.Join(missing, "\n  "))
		}
		time.Sleep(10 * time.Second)
	}
}

func (h *Harness) indexedARNs(ctx context.Context) (map[string]bool, error) {
	found := map[string]bool{}
	p := rgt.NewGetResourcesPaginator(h.rgt, &rgt.GetResourcesInput{
		TagFilters: []rgttypes.TagFilter{{
			Key:    aws.String(RunTagKey),
			Values: []string{h.RunID},
		}},
	})
	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, m := range out.ResourceTagMappingList {
			found[aws.ToString(m.ResourceARN)] = true
		}
	}
	return found, nil
}
