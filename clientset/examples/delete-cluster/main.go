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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	ctx := context.Background()

	host := util.MustEnv("HYPERFLEET_HOST")
	id   := util.MustEnv("HYPERFLEET_CLUSTER_ID")

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

	if err := clusters.Delete(ctx, id, metav1.DeleteOptions{}); err != nil {
		log.Fatalf("deleting cluster: %v", err)
	}

	fmt.Printf("Cluster %s deletion initiated, waiting for removal...\n", id)

	err = clusters.WaitUntil(ctx, id,
		func(c *v1alpha1.Cluster) bool {
			if c == nil {
				return true
			}
			log.Printf("cluster %s: phase=%s, waiting...", id, c.Status.Phase)
			return false
		},
		10*time.Second, 30*time.Minute,
	)
	if err != nil {
		log.Fatalf("waiting for cluster deletion: %v", err)
	}

	fmt.Printf("Cluster %s deleted.\n", id)
}
