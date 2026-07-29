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

// patch updates the replica count of an existing NodePool.
//
// The pattern is get → modify → update: read the full current object first so
// all unrelated fields are preserved in the PUT body.
//
// Required environment variables:
//
//	HYPERFLEET_HOST        — platform API base URL
//	HYPERFLEET_CLUSTER_ID  — cluster UUID
//	HYPERFLEET_NODEPOOL_ID — node pool UUID
//	HYPERFLEET_REPLICAS    — desired replica count
package main

import (
	"context"
	"fmt"
	"log"
	"strconv"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	hyperfleet "github.com/openshift-online/rosa-hyperfleet-api/clientset"
	"github.com/openshift-online/rosa-hyperfleet-api/clientset/examples/util"
	hfrest "github.com/openshift-online/rosa-hyperfleet-api/clientset/rest"
	"github.com/openshift-online/rosa-hyperfleet-api/clientset/wrappers"
)

func main() {
	ctx := context.Background()

	host := util.MustEnv("HYPERFLEET_HOST")
	clusterID := util.MustEnv("HYPERFLEET_CLUSTER_ID")
	nodepoolID := util.MustEnv("HYPERFLEET_NODEPOOL_ID")

	replicasStr := util.MustEnv("HYPERFLEET_REPLICAS")
	replicasN, err := strconv.Atoi(replicasStr)
	if err != nil {
		log.Fatalf("HYPERFLEET_REPLICAS must be an integer: %v", err)
	}
	replicas := int32(replicasN)

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

	np, err := nodepools.Get(ctx, nodepoolID, wrappers.GetOptions{})
	if err != nil {
		log.Fatalf("getting node pool: %v", err)
	}

	np.Spec.NodePool.Replicas = &replicas

	updated, err := nodepools.Update(ctx, np, wrappers.UpdateOptions{})
	if err != nil {
		log.Fatalf("updating node pool: %v", err)
	}

	fmt.Printf("NodePool %s updated: replicas=%d\n", nodepoolID, *updated.Spec.NodePool.Replicas)
}
