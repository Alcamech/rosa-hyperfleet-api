package validation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
)

const additionalAllowedPrincipalsPath = "spec.hostedCluster.platform.aws.additionalAllowedPrincipals"

var (
	awsAccountIDPattern = regexp.MustCompile(`^[0-9]{12}$`)
	awsPartitionPattern = regexp.MustCompile(`^aws(-[a-z0-9]+)*$`)
	iamPrincipalName    = regexp.MustCompile(`^[A-Za-z0-9+=,.@_-]+(/[A-Za-z0-9+=,.@_-]+)*$`)
)

// validateAdditionalAllowedPrincipals validates the customer-managed principal
// list accepted by HyperShift's AWS endpoint-service reconciliation.
func validateAdditionalAllowedPrincipals(fields map[string]any) *ValidationError {
	value, present := fields[additionalAllowedPrincipalsPath]
	if !present {
		return nil
	}

	principals, ok := value.([]any)
	if !ok {
		return invalidPrincipalList("must be an array of IAM principal ARNs")
	}
	if len(principals) > 25 {
		return invalidPrincipalList("cannot contain more than 25 principals")
	}

	seen := make(map[string]struct{}, len(principals))
	for i, value := range principals {
		principal, ok := value.(string)
		if !ok || !validAdditionalAllowedPrincipal(principal) {
			return invalidPrincipalList(fmt.Sprintf("entry %d must be an AWS IAM account, role, user, or federated-user ARN", i))
		}
		if len(principal) > 255 {
			return invalidPrincipalList(fmt.Sprintf("entry %d exceeds the 255-character ARN limit", i))
		}
		if _, duplicate := seen[principal]; duplicate {
			return invalidPrincipalList(fmt.Sprintf("entry %d duplicates principal %q", i, principal))
		}
		seen[principal] = struct{}{}
	}
	return nil
}

func validAdditionalAllowedPrincipal(value string) bool {
	parsed, err := arn.Parse(value)
	if err != nil || !awsPartitionPattern.MatchString(parsed.Partition) || parsed.Service != "iam" || parsed.Region != "" ||
		!awsAccountIDPattern.MatchString(parsed.AccountID) {
		return false
	}

	if parsed.Resource == "root" {
		return true
	}
	for _, resourceType := range []string{"role", "user", "federated-user"} {
		prefix := resourceType + "/"
		if strings.HasPrefix(parsed.Resource, prefix) {
			return iamPrincipalName.MatchString(strings.TrimPrefix(parsed.Resource, prefix))
		}
	}
	return false
}

func invalidPrincipalList(reason string) *ValidationError {
	return &ValidationError{
		Field:  additionalAllowedPrincipalsPath,
		Reason: reason,
	}
}
