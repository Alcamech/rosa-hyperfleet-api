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

// create creates a NodePool in a ROSA HCP cluster and waits until it is Ready.
//
// Required environment variables:
//
//	HYPERFLEET_HOST           — platform API base URL
//	HYPERFLEET_CLUSTER_ID     — cluster UUID (parent cluster)
//	HYPERFLEET_CLUSTER_NAME   — cluster name (used as ClusterName in the NodePool spec)
//	HYPERFLEET_NODEPOOL_NAME  — human-readable name for the new node pool
//	HYPERFLEET_VERSION        — OpenShift release image (e.g. "5.0.0-ec.2")
//	HYPERFLEET_SUBNET_ID      — private subnet ID for the node instances
//	HYPERFLEET_INSTANCE_TYPE  — EC2 instance type (e.g. "m5.large")
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
	"github.com/openshift-online/rosa-hyperfleet-api/clientset/wrappers"
	v1alpha1 "github.com/openshift-online/rosa-hyperfleet-api/hyperfleet-operator/api/v1alpha1"
	hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	ctx := context.Background()

	host := util.MustEnv("HYPERFLEET_HOST")
	clusterID := util.MustEnv("HYPERFLEET_CLUSTER_ID")
	clusterName := util.MustEnv("HYPERFLEET_CLUSTER_NAME")
	name := util.MustEnv("HYPERFLEET_NODEPOOL_NAME")
	version := util.MustEnv("HYPERFLEET_VERSION")
	subnetID := util.MustEnv("HYPERFLEET_SUBNET_ID")
	instanceType := util.MustEnv("HYPERFLEET_INSTANCE_TYPE")

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("loading AWS config: %v", err)
	}

	identity, err := sts.NewFromConfig(awsCfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Fatalf("getting caller identity: %v", err)
	}

	cs, err := hyperfleet.NewForConfig(&hfrest.Config{
		Host:      host,
		AccountID: *identity.Account,
		CallerARN: *identity.Arn,
		AWSConfig: awsCfg,
	})
	if err != nil {
		log.Fatalf("building clientset: %v", err)
	}

	nodepools := cs.HyperfleetV1alpha1().NodePools(clusterID)

	replicas := int32(2)
	np, err := nodepools.Create(ctx, &v1alpha1.NodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.NodePoolSpec{
			NodePool: hypershiftv1beta1.NodePoolSpec{
				ClusterName: clusterName,
				Release:     hypershiftv1beta1.Release{Image: version},
				Replicas:    &replicas,
				Platform: hypershiftv1beta1.NodePoolPlatform{
					Type: hypershiftv1beta1.AWSPlatform,
					AWS: &hypershiftv1beta1.AWSNodePoolPlatform{
						InstanceType: instanceType,
						Subnet:       hypershiftv1beta1.AWSResourceReference{ID: &subnetID},
					},
				},
				Management: hypershiftv1beta1.NodePoolManagement{
					UpgradeType: hypershiftv1beta1.UpgradeTypeReplace,
				},
			},
		},
	}, wrappers.CreateOptions{})
	if err != nil {
		log.Fatalf("creating node pool: %v", err)
	}

	id := string(np.UID)
	fmt.Printf("NodePool %s created (id=%s), waiting until Ready...\n", name, id)

	err = nodepools.WaitUntil(ctx, id,
		func(np *v1alpha1.NodePool) bool {
			if np == nil {
				log.Printf("node pool %s: disappeared unexpectedly", id)
				return true
			}
			log.Printf("node pool %s: phase=%s, waiting...", name, np.Status.Phase)
			return np.Status.Phase == v1alpha1.NodePoolPhaseReady
		},
		15*time.Second, 30*time.Minute,
	)
	if err != nil {
		log.Fatalf("waiting for node pool to be ready: %v", err)
	}

	fmt.Printf("NodePool %s is Ready.\n", name)
}
