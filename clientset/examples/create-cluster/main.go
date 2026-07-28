/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// create-cluster creates a ROSA HCP cluster and waits until it reaches the Ready phase.
//
// Required environment variables:
//
//	HYPERFLEET_HOST         — platform API base URL
//	HYPERFLEET_CLUSTER_NAME — human-readable cluster name
//	HYPERFLEET_VERSION      — OpenShift release image (e.g. "5.0.0-ec.2")
//	HYPERFLEET_VPC_ID       — VPC ID for the control plane
//	HYPERFLEET_SUBNET_ID    — private subnet ID for the control plane
//
// IAM roles are derived from the cluster name and AWS account ID using the
// same naming convention as the rosactl CLI:
//
//	arn:aws:iam::{accountID}:role/{clusterName}-{roleSuffix}
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	hyperfleet "github.com/openshift-online/rosa-hyperfleet-api/clientset"
	"github.com/openshift-online/rosa-hyperfleet-api/clientset/examples/util"
	hfrest "github.com/openshift-online/rosa-hyperfleet-api/clientset/rest"
	v1alpha1 "github.com/openshift-online/rosa-hyperfleet-api/hyperfleet-operator/api/v1alpha1"
	hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	ctx := context.Background()

	host := util.MustEnv("HYPERFLEET_HOST")
	name := util.MustEnv("HYPERFLEET_CLUSTER_NAME")
	vpcID := util.MustEnv("HYPERFLEET_VPC_ID")
	subnetID := util.MustEnv("HYPERFLEET_SUBNET_ID")
	version := util.MustEnv("HYPERFLEET_VERSION")

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("loading AWS config: %v", err)
	}

	identity, err := sts.NewFromConfig(awsCfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Fatalf("getting caller identity: %v", err)
	}
	accountID := *identity.Account

	cfg := &hfrest.Config{
		Host:      host,
		AccountID: accountID,
		CallerARN: *identity.Arn,
		AWSConfig: awsCfg,
	}

	region, err := cfg.ResolveRegion()
	if err != nil {
		log.Fatalf("resolving region from host: %v", err)
	}

	cs, err := hyperfleet.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("building clientset: %v", err)
	}

	clusters := cs.HyperfleetV1alpha1().Clusters(accountID)

	cluster, err := clusters.Create(ctx, &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.ClusterSpec{
			HostedCluster: hypershiftv1beta1.HostedClusterSpec{
				Release: hypershiftv1beta1.Release{
					Image: version,
				},
				Platform: hypershiftv1beta1.PlatformSpec{
					Type: hypershiftv1beta1.AWSPlatform,
					AWS: &hypershiftv1beta1.AWSPlatformSpec{
						Region:   region,
						RolesRef: iamRoles(name, accountID),
						CloudProviderConfig: &hypershiftv1beta1.AWSCloudProviderConfig{
							VPC:    vpcID,
							Zone:   region + "a",
							Subnet: &hypershiftv1beta1.AWSResourceReference{ID: &subnetID},
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("creating cluster: %v", err)
	}

	id := string(cluster.UID)
	fmt.Printf("Cluster %s created (id=%s), waiting until Ready...\n", name, id)

	err = clusters.WaitUntil(ctx, id,
		func(c *v1alpha1.Cluster) bool {
			if c == nil {
				log.Printf("cluster %s: disappeared unexpectedly", id)
				return true
			}
			log.Printf("cluster %s: phase=%s, waiting...", name, c.Status.Phase)
			return c.Status.Phase == v1alpha1.ClusterPhaseReady
		},
		15*time.Second, 30*time.Minute,
	)
	if err != nil {
		log.Fatalf("waiting for cluster to be ready: %v", err)
	}

	fmt.Printf("Cluster %s is Ready.\n", name)
}

// iamRoles computes IAM role ARNs from the cluster name and account ID using
// the same naming convention as the rosactl CLI.
func iamRoles(clusterName, accountID string) hypershiftv1beta1.AWSRolesRef {
	arn := func(suffix string) string {
		return fmt.Sprintf("arn:aws:iam::%s:role/%s-%s", accountID, clusterName, suffix)
	}
	return hypershiftv1beta1.AWSRolesRef{
		IngressARN:              arn("ingress"),
		ImageRegistryARN:        arn("image-registry"),
		StorageARN:              arn("ebs-csi"),
		NetworkARN:              arn("network-config"),
		KubeCloudControllerARN:  arn("cloud-controller-manager"),
		NodePoolManagementARN:   arn("node-pool-management"),
		ControlPlaneOperatorARN: arn("control-plane-operator"),
	}
}
