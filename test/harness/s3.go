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
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"
)

// CreateBucket makes an empty bucket, whose ARN carries neither region nor account.
func CreateBucket(ctx context.Context, t *testing.T, h *Harness) (string, map[string]string) {
	name := h.ResourceName()
	c := s3.NewFromConfig(h.Cfg)

	in := &s3.CreateBucketInput{Bucket: &name}
	if h.Region != "us-east-1" {
		in.CreateBucketConfiguration = &s3types.CreateBucketConfiguration{
			LocationConstraint: s3types.BucketLocationConstraint(h.Region),
		}
	}
	_, err := c.CreateBucket(ctx, in)
	require.NoError(t, err, "creating bucket %s", name)
	t.Cleanup(func() {
		_, derr := c.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: &name})
		CleanupErr(t, derr, "s3 bucket "+name)
	})

	_, err = c.PutBucketTagging(ctx, &s3.PutBucketTaggingInput{
		Bucket: &name,
		Tagging: &s3types.Tagging{TagSet: []s3types.Tag{{
			Key:   aws.String(RunTagKey),
			Value: aws.String(h.RunID),
		}}},
	})
	require.NoError(t, err, "tagging bucket %s", name)

	// S3 returns no ARN and its form is fixed, so the partition comes from the caller.
	return "arn:" + h.Partition + ":s3:::" + name, map[string]string{"name": name}
}
