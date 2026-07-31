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

package hyperfleet

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	hfrest "github.com/openshift-online/rosa-hyperfleet-api/clientset/rest"
)

func staticCreds() aws.CredentialsProvider {
	return credentials.NewStaticCredentialsProvider("AKIATEST", "secretkey", "")
}

func validConfig() *hfrest.Config {
	return &hfrest.Config{
		Host:      "https://abc123.execute-api.us-east-1.amazonaws.com/prod",
		AccountID: "123456789012",
		AWSConfig: aws.Config{
			Region:      "us-east-1",
			Credentials: staticCreds(),
		},
	}
}

func TestNewForConfig_NilCfg(t *testing.T) {
	if _, err := NewForConfig(nil); err == nil {
		t.Error("expected error for nil cfg")
	}
}

func TestNewForConfig_NilCredentials(t *testing.T) {
	cfg := validConfig()
	cfg.AWSConfig = aws.Config{Region: "us-east-1"} // Credentials is nil
	if _, err := NewForConfig(cfg); err == nil {
		t.Error("expected error when AWSConfig.Credentials is nil")
	}
}

func TestNewForConfig_RequiresHost(t *testing.T) {
	cfg := validConfig()
	cfg.Host = ""
	if _, err := NewForConfig(cfg); err == nil {
		t.Error("expected error when Host is empty")
	}
}

func TestNewForConfig_RequiresAccountID(t *testing.T) {
	cfg := validConfig()
	cfg.AccountID = ""
	if _, err := NewForConfig(cfg); err == nil {
		t.Error("expected error when AccountID is empty")
	}
}

func TestNewForConfig_RegionDerivedFromExecuteAPIHost(t *testing.T) {
	cfg := validConfig()
	// Clear explicit region; must be derived from Host.
	cfg.AWSConfig = aws.Config{Credentials: staticCreds()}
	if _, err := NewForConfig(cfg); err != nil {
		t.Errorf("expected success with derivable region from Host: %v", err)
	}
}

func TestNewForConfig_ErrorsWhenRegionCannotBeDerived(t *testing.T) {
	cfg := validConfig()
	cfg.Host = "https://example.com/api"
	cfg.AWSConfig = aws.Config{Credentials: staticCreds()} // no region, host has none
	if _, err := NewForConfig(cfg); err == nil {
		t.Error("expected error when region cannot be derived from Host")
	}
}

func TestNewForConfig_ExplicitRegionTakesPrecedence(t *testing.T) {
	cfg := validConfig()
	cfg.Region = "eu-west-1"
	cfg.Host = "https://example.com/api"
	cfg.AWSConfig = aws.Config{Credentials: staticCreds()} // explicit Region, no derivation
	if _, err := NewForConfig(cfg); err != nil {
		t.Errorf("expected success with explicit Region: %v", err)
	}
}
