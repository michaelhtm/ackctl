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

// Package integration runs the AWS-only suite: it needs credentials and a region, but no
// cluster.
package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/aws-controllers-k8s/ackctl/test/harness"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAdoptIsDeterministic checks that identical runs produce identical YAML, so `kubectl
// create` rejects a second adoption of the same resource by name.
func TestAdoptIsDeterministic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	h := harness.New(ctx, t)

	arnStr, _ := harness.CreateBucket(ctx, t, h)
	h.WaitForTagIndex(ctx, t, map[string]string{arnStr: "s3/Bucket"})

	first, _ := h.Adopt(t, "s3", "Bucket")
	second, _ := h.Adopt(t, "s3", "Bucket")
	require.Equal(t, first, second,
		"two identical runs produced different YAML; re-running would create a second CR "+
			"for the same AWS resource")
	require.NotEmpty(t, first)
}

// TestAdoptExplainsAnEmptyResult covers the outcome users hit most often. The summary
// names the region, filter and selector; the sweep that tells a wrong region from a wrong
// tag costs a second query, so it is left to -v1.
func TestAdoptExplainsAnEmptyResult(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	h := harness.New(ctx, t)

	stdout, stderr := h.Adopt(t, "s3", "Bucket")
	assert.Empty(t, strings.TrimSpace(stdout),
		"an empty result must emit no YAML at all, or a pipe into kubectl would apply nothing "+
			"while looking like it worked")
	assert.Contains(t, stderr, "no s3/Bucket resources matched")
	assert.Contains(t, stderr, h.Region, "the summary should name the region searched")
	assert.Contains(t, stderr, "s3:bucket", "the type filter used should be visible")
	assert.Contains(t, stderr, "-v1",
		"the default path should point at the sweep rather than run it")
}
