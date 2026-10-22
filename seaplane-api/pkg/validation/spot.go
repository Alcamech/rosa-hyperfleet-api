package validation

import "regexp"

const (
	terminationHandlerQueueURLField = "spec.hostedCluster.platform.aws.terminationHandlerQueueURL"
	maxTerminationHandlerQueueURL   = 512
)

var terminationHandlerQueueURLPattern = regexp.MustCompile(`^https://sqs\.([a-z0-9-]+)\.amazonaws\.com/[0-9]{12}/([A-Za-z0-9_-]{1,80}|[A-Za-z0-9_-]{1,75}\.fifo)$`)

// ValidateSpotTerminationHandlerQueueURL validates the optional customer-owned
// SQS queue used for graceful Spot interruption handling. An empty URL is valid:
// Spot NodePools still work in simple mode without a termination handler.
func ValidateSpotTerminationHandlerQueueURL(queueURL, region string, existingQueueURLs []string) ValidationErrors {
	if queueURL == "" {
		return nil
	}

	matches := terminationHandlerQueueURLPattern.FindStringSubmatch(queueURL)
	if matches == nil || len(queueURL) > maxTerminationHandlerQueueURL {
		return ValidationErrors{&ValidationError{
			Field:  terminationHandlerQueueURLField,
			Reason: "must be an HTTPS SQS queue URL in the form https://sqs.<region>.amazonaws.com/<12-digit-account>/<queue-name>",
		}}
	}

	var errs ValidationErrors
	if region == "" || matches[1] != region {
		errs = append(errs, &ValidationError{
			Field:  terminationHandlerQueueURLField,
			Reason: "must use the same AWS region as the cluster",
		})
	}

	for _, existingURL := range existingQueueURLs {
		if queueURL == existingURL {
			errs = append(errs, &ValidationError{
				Field:  terminationHandlerQueueURLField,
				Reason: "must be unique across clusters in the account",
			})
			break
		}
	}

	return errs
}

// SpotTerminationHandlerQueueAlreadyInUse returns a field validation error for
// the atomic reservation conflict that can occur after a list-based precheck.
func SpotTerminationHandlerQueueAlreadyInUse() ValidationErrors {
	return ValidationErrors{&ValidationError{
		Field:  terminationHandlerQueueURLField,
		Reason: "must be unique across clusters in the account",
	}}
}
