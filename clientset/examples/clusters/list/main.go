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

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	hyperfleet "github.com/openshift-online/rosa-hyperfleet-api/clientset"
	"github.com/openshift-online/rosa-hyperfleet-api/clientset/examples/util"
	hfrest "github.com/openshift-online/rosa-hyperfleet-api/clientset/rest"
	"github.com/openshift-online/rosa-hyperfleet-api/clientset/wrappers"
)

const pageSize = 10

func main() {
	ctx := context.Background()

	host := util.MustEnv("HYPERFLEET_HOST")

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

	client := cs.HyperfleetV1alpha1().Clusters(*identity.Account)

	var total int
	for offset := int64(0); ; offset += pageSize {
		page, err := client.List(ctx, wrappers.ListOptions{Limit: pageSize, Offset: offset})
		if err != nil {
			log.Fatalf("listing clusters (offset=%d): %v", offset, err)
		}

		fmt.Printf("Page offset=%-4d  items=%d\n", offset, len(page.Items))
		for _, c := range page.Items {
			fmt.Printf("  %-30s  uid=%-40s  phase=%-20s  version=%s\n",
				c.Name,
				c.UID,
				c.Status.Phase,
				c.Status.Version,
			)
		}

		total += len(page.Items)
		if len(page.Items) == 0 {
			break
		}
	}

	fmt.Printf("Total: %d cluster(s)\n", total)
}
