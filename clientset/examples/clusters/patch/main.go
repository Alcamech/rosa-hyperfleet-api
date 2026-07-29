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

// patch-cluster updates the autoscaling configuration of an existing ROSA HCP cluster,
// enabling scale-up and scale-down with a 5-minute delay after node addition.
//
// The pattern is get → modify → update: read the full current object first so
// all unrelated fields are preserved in the PUT body.
//
// Required environment variables:
//
//	HYPERFLEET_HOST       — platform API base URL
//	HYPERFLEET_CLUSTER_ID — cluster UUID
package main

import (
	"context"
	"fmt"
	"log"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	hyperfleet "github.com/openshift-online/rosa-hyperfleet-api/clientset"
	"github.com/openshift-online/rosa-hyperfleet-api/clientset/examples/util"
	hfrest "github.com/openshift-online/rosa-hyperfleet-api/clientset/rest"
	"github.com/openshift-online/rosa-hyperfleet-api/clientset/wrappers"
	hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
)

func main() {
	ctx := context.Background()

	host := util.MustEnv("HYPERFLEET_HOST")
	id := util.MustEnv("HYPERFLEET_CLUSTER_ID")

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

	clusters := cs.HyperfleetV1alpha1().Clusters(*identity.Account)

	cluster, err := clusters.Get(ctx, id, wrappers.GetOptions{})
	if err != nil {
		log.Fatalf("getting cluster: %v", err)
	}

	delayAfterAdd := int32(300)
	cluster.Spec.HostedCluster.Autoscaling = hypershiftv1beta1.ClusterAutoscaling{
		Scaling: hypershiftv1beta1.ScaleUpAndScaleDown,
		ScaleDown: &hypershiftv1beta1.ScaleDownConfig{
			DelayAfterAddSeconds: &delayAfterAdd,
		},
	}

	updated, err := clusters.Update(ctx, cluster, wrappers.UpdateOptions{})
	if err != nil {
		log.Fatalf("updating cluster: %v", err)
	}

	fmt.Printf("Cluster %s updated: scaling=%s, delayAfterAdd=%ds\n",
		id,
		updated.Spec.HostedCluster.Autoscaling.Scaling,
		*updated.Spec.HostedCluster.Autoscaling.ScaleDown.DelayAfterAddSeconds,
	)
}
