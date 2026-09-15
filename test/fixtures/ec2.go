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

package fixtures

import (
	"context"
	"testing"

	"github.com/aws-controllers-k8s/ackctl/test/harness"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/require"
)

// ec2Tags is the run tag in the shape CreateTags-on-create expects.
func (f *Fixtures) ec2Tags(resource ec2types.ResourceType) []ec2types.TagSpecification {
	return f.ec2TagsWithKey(resource, harness.RunTagKey)
}

// ec2SharedTags tags a shared prerequisite so it is recognisable but never selected.
func (f *Fixtures) ec2SharedTags(resource ec2types.ResourceType) []ec2types.TagSpecification {
	return f.ec2TagsWithKey(resource, harness.SharedTagKey)
}

func (f *Fixtures) ec2TagsWithKey(resource ec2types.ResourceType, key string) []ec2types.TagSpecification {
	return []ec2types.TagSpecification{{
		ResourceType: resource,
		Tags: []ec2types.Tag{
			{Key: aws.String(key), Value: aws.String(f.RunID)},
			{Key: aws.String("Name"), Value: aws.String(f.ResourceName())},
		},
	}}
}

func (f *Fixtures) ec2() *ec2.Client { return ec2.NewFromConfig(f.Cfg) }

// ec2ARN builds an EC2 ARN, which most EC2 create calls do not return.
func (f *Fixtures) ec2ARN(resourceType, id string) string {
	return "arn:" + f.Partition + ":ec2:" + f.Region + ":" + f.Account + ":" + resourceType + "/" + id
}

// sharedVPC creates one VPC the whole run reuses, since several kinds need a parent VPC.
func (f *Fixtures) sharedVPC(ctx context.Context, t *testing.T) string {
	t.Helper()
	if f.vpcID != "" {
		return f.vpcID
	}
	c := f.ec2()
	out, err := c.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock:         aws.String("10.220.0.0/16"),
		TagSpecifications: f.ec2SharedTags(ec2types.ResourceTypeVpc),
	})
	require.NoError(t, err, "creating the shared VPC")
	id := aws.ToString(out.Vpc.VpcId)
	t.Cleanup(func() {
		_, derr := c.DeleteVpc(context.Background(), &ec2.DeleteVpcInput{VpcId: aws.String(id)})
		harness.CleanupErr(t, derr, "ec2 vpc "+id)
	})
	f.vpcID = id
	return id
}

func createVPC(ctx context.Context, t *testing.T, f *Fixtures) (string, map[string]string) {
	c := f.ec2()
	out, err := c.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock:         aws.String("10.221.0.0/16"),
		TagSpecifications: f.ec2Tags(ec2types.ResourceTypeVpc),
	})
	require.NoError(t, err, "creating VPC")
	id := aws.ToString(out.Vpc.VpcId)
	t.Cleanup(func() {
		_, derr := c.DeleteVpc(context.Background(), &ec2.DeleteVpcInput{VpcId: aws.String(id)})
		harness.CleanupErr(t, derr, "ec2 vpc "+id)
	})
	return f.ec2ARN("vpc", id), map[string]string{"vpcID": id}
}

func createSubnet(ctx context.Context, t *testing.T, f *Fixtures) (string, map[string]string) {
	c := f.ec2()
	out, err := c.CreateSubnet(ctx, &ec2.CreateSubnetInput{
		VpcId:             aws.String(f.sharedVPC(ctx, t)),
		CidrBlock:         aws.String("10.220.1.0/24"),
		TagSpecifications: f.ec2Tags(ec2types.ResourceTypeSubnet),
	})
	require.NoError(t, err, "creating subnet")
	id := aws.ToString(out.Subnet.SubnetId)
	t.Cleanup(func() {
		_, derr := c.DeleteSubnet(context.Background(), &ec2.DeleteSubnetInput{SubnetId: aws.String(id)})
		harness.CleanupErr(t, derr, "ec2 subnet "+id)
	})
	return f.ec2ARN("subnet", id), map[string]string{"subnetID": id}
}

func createSecurityGroup(ctx context.Context, t *testing.T, f *Fixtures) (string, map[string]string) {
	c := f.ec2()
	out, err := c.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:         aws.String(f.ResourceName()),
		Description:       aws.String("ackctl integration test"),
		VpcId:             aws.String(f.sharedVPC(ctx, t)),
		TagSpecifications: f.ec2Tags(ec2types.ResourceTypeSecurityGroup),
	})
	require.NoError(t, err, "creating security group")
	id := aws.ToString(out.GroupId)
	t.Cleanup(func() {
		_, derr := c.DeleteSecurityGroup(context.Background(),
			&ec2.DeleteSecurityGroupInput{GroupId: aws.String(id)})
		harness.CleanupErr(t, derr, "ec2 security group "+id)
	})
	return f.ec2ARN("security-group", id), map[string]string{"id": id}
}

func createInternetGateway(ctx context.Context, t *testing.T, f *Fixtures) (string, map[string]string) {
	c := f.ec2()
	out, err := c.CreateInternetGateway(ctx, &ec2.CreateInternetGatewayInput{
		TagSpecifications: f.ec2Tags(ec2types.ResourceTypeInternetGateway),
	})
	require.NoError(t, err, "creating internet gateway")
	id := aws.ToString(out.InternetGateway.InternetGatewayId)
	t.Cleanup(func() {
		_, derr := c.DeleteInternetGateway(context.Background(),
			&ec2.DeleteInternetGatewayInput{InternetGatewayId: aws.String(id)})
		harness.CleanupErr(t, derr, "ec2 internet gateway "+id)
	})
	return f.ec2ARN("internet-gateway", id), map[string]string{"internetGatewayID": id}
}

func createEgressOnlyInternetGateway(ctx context.Context, t *testing.T, f *Fixtures) (string, map[string]string) {
	c := f.ec2()
	out, err := c.CreateEgressOnlyInternetGateway(ctx, &ec2.CreateEgressOnlyInternetGatewayInput{
		VpcId:             aws.String(f.sharedVPC(ctx, t)),
		TagSpecifications: f.ec2Tags(ec2types.ResourceTypeEgressOnlyInternetGateway),
	})
	require.NoError(t, err, "creating egress-only internet gateway")
	id := aws.ToString(out.EgressOnlyInternetGateway.EgressOnlyInternetGatewayId)
	t.Cleanup(func() {
		_, derr := c.DeleteEgressOnlyInternetGateway(context.Background(),
			&ec2.DeleteEgressOnlyInternetGatewayInput{EgressOnlyInternetGatewayId: aws.String(id)})
		harness.CleanupErr(t, derr, "ec2 egress-only internet gateway "+id)
	})
	return f.ec2ARN("egress-only-internet-gateway", id), map[string]string{"id": id}
}

func createRouteTable(ctx context.Context, t *testing.T, f *Fixtures) (string, map[string]string) {
	c := f.ec2()
	out, err := c.CreateRouteTable(ctx, &ec2.CreateRouteTableInput{
		VpcId:             aws.String(f.sharedVPC(ctx, t)),
		TagSpecifications: f.ec2Tags(ec2types.ResourceTypeRouteTable),
	})
	require.NoError(t, err, "creating route table")
	id := aws.ToString(out.RouteTable.RouteTableId)
	t.Cleanup(func() {
		_, derr := c.DeleteRouteTable(context.Background(),
			&ec2.DeleteRouteTableInput{RouteTableId: aws.String(id)})
		harness.CleanupErr(t, derr, "ec2 route table "+id)
	})
	return f.ec2ARN("route-table", id), map[string]string{"routeTableID": id}
}

func createNetworkACL(ctx context.Context, t *testing.T, f *Fixtures) (string, map[string]string) {
	c := f.ec2()
	out, err := c.CreateNetworkAcl(ctx, &ec2.CreateNetworkAclInput{
		VpcId:             aws.String(f.sharedVPC(ctx, t)),
		TagSpecifications: f.ec2Tags(ec2types.ResourceTypeNetworkAcl),
	})
	require.NoError(t, err, "creating network ACL")
	id := aws.ToString(out.NetworkAcl.NetworkAclId)
	t.Cleanup(func() {
		_, derr := c.DeleteNetworkAcl(context.Background(),
			&ec2.DeleteNetworkAclInput{NetworkAclId: aws.String(id)})
		harness.CleanupErr(t, derr, "ec2 network acl "+id)
	})
	return f.ec2ARN("network-acl", id), map[string]string{"id": id}
}

func createDHCPOptions(ctx context.Context, t *testing.T, f *Fixtures) (string, map[string]string) {
	c := f.ec2()
	out, err := c.CreateDhcpOptions(ctx, &ec2.CreateDhcpOptionsInput{
		DhcpConfigurations: []ec2types.NewDhcpConfiguration{{
			Key:    aws.String("domain-name-servers"),
			Values: []string{"10.220.0.2"},
		}},
		TagSpecifications: f.ec2Tags(ec2types.ResourceTypeDhcpOptions),
	})
	require.NoError(t, err, "creating DHCP options")
	id := aws.ToString(out.DhcpOptions.DhcpOptionsId)
	t.Cleanup(func() {
		_, derr := c.DeleteDhcpOptions(context.Background(),
			&ec2.DeleteDhcpOptionsInput{DhcpOptionsId: aws.String(id)})
		harness.CleanupErr(t, derr, "ec2 dhcp options "+id)
	})
	return f.ec2ARN("dhcp-options", id), map[string]string{"dhcpOptionsID": id}
}

func createLaunchTemplate(ctx context.Context, t *testing.T, f *Fixtures) (string, map[string]string) {
	c := f.ec2()
	out, err := c.CreateLaunchTemplate(ctx, &ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: aws.String(f.ResourceName()),
		LaunchTemplateData: &ec2types.RequestLaunchTemplateData{
			InstanceType: ec2types.InstanceTypeT3Micro,
		},
		TagSpecifications: f.ec2Tags(ec2types.ResourceTypeLaunchTemplate),
	})
	require.NoError(t, err, "creating launch template")
	id := aws.ToString(out.LaunchTemplate.LaunchTemplateId)
	t.Cleanup(func() {
		_, derr := c.DeleteLaunchTemplate(context.Background(),
			&ec2.DeleteLaunchTemplateInput{LaunchTemplateId: aws.String(id)})
		harness.CleanupErr(t, derr, "ec2 launch template "+id)
	})
	return f.ec2ARN("launch-template", id), map[string]string{"id": id}
}

func createManagedPrefixList(ctx context.Context, t *testing.T, f *Fixtures) (string, map[string]string) {
	c := f.ec2()
	out, err := c.CreateManagedPrefixList(ctx, &ec2.CreateManagedPrefixListInput{
		PrefixListName:    aws.String(f.ResourceName()),
		AddressFamily:     aws.String("IPv4"),
		MaxEntries:        aws.Int32(5),
		TagSpecifications: f.ec2Tags(ec2types.ResourceTypePrefixList),
	})
	require.NoError(t, err, "creating managed prefix list")
	id := aws.ToString(out.PrefixList.PrefixListId)
	t.Cleanup(func() {
		_, derr := c.DeleteManagedPrefixList(context.Background(),
			&ec2.DeleteManagedPrefixListInput{PrefixListId: aws.String(id)})
		harness.CleanupErr(t, derr, "ec2 managed prefix list "+id)
	})
	return f.ec2ARN("prefix-list", id), map[string]string{"id": id}
}

// twoSubnets creates the two-AZ pair that subnet-group kinds require, once per run.
func (f *Fixtures) twoSubnets(ctx context.Context, t *testing.T) []string {
	t.Helper()
	if len(f.subnetIDs) > 0 {
		return f.subnetIDs
	}
	c := f.ec2()
	azs, err := c.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{})
	require.NoError(t, err, "listing availability zones")
	require.GreaterOrEqual(t, len(azs.AvailabilityZones), 2, "need two AZs for a subnet group")

	vpcID := f.sharedVPC(ctx, t)
	for i, cidr := range []string{"10.220.10.0/24", "10.220.11.0/24"} {
		out, serr := c.CreateSubnet(ctx, &ec2.CreateSubnetInput{
			VpcId:             aws.String(vpcID),
			CidrBlock:         aws.String(cidr),
			AvailabilityZone:  azs.AvailabilityZones[i].ZoneName,
			TagSpecifications: f.ec2SharedTags(ec2types.ResourceTypeSubnet),
		})
		require.NoError(t, serr, "creating shared subnet %s", cidr)
		id := aws.ToString(out.Subnet.SubnetId)
		t.Cleanup(func() {
			_, derr := c.DeleteSubnet(context.Background(), &ec2.DeleteSubnetInput{SubnetId: aws.String(id)})
			harness.CleanupErr(t, derr, "ec2 subnet "+id)
		})
		f.subnetIDs = append(f.subnetIDs, id)
	}
	return f.subnetIDs
}
