package validation

import (
	"strings"
	"testing"
)

func TestValidateSpotTerminationHandlerQueueURL(t *testing.T) {
	const validQueueURL = "https://sqs.us-east-1.amazonaws.com/123456789012/spot-termination"

	tests := []struct {
		name             string
		queueURL         string
		region           string
		existingQueueURL []string
		wantError        bool
	}{
		{
			name:     "queue is optional",
			region:   "us-east-1",
			queueURL: "",
		},
		{
			name:     "standard queue URL",
			queueURL: validQueueURL,
			region:   "us-east-1",
		},
		{
			name:     "FIFO queue URL",
			queueURL: validQueueURL + ".fifo",
			region:   "us-east-1",
		},
		{
			name:     "standard queue name at maximum length",
			queueURL: validQueueURL[:strings.LastIndex(validQueueURL, "/")+1] + strings.Repeat("a", 80),
			region:   "us-east-1",
		},
		{
			name:      "standard queue name exceeds maximum length",
			queueURL:  validQueueURL[:strings.LastIndex(validQueueURL, "/")+1] + strings.Repeat("a", 81),
			region:    "us-east-1",
			wantError: true,
		},
		{
			name:     "FIFO queue name at maximum length",
			queueURL: validQueueURL[:strings.LastIndex(validQueueURL, "/")+1] + strings.Repeat("a", 75) + ".fifo",
			region:   "us-east-1",
		},
		{
			name:      "FIFO queue name exceeds maximum length",
			queueURL:  validQueueURL[:strings.LastIndex(validQueueURL, "/")+1] + strings.Repeat("a", 76) + ".fifo",
			region:    "us-east-1",
			wantError: true,
		},
		{
			name:      "invalid URL format",
			queueURL:  "http://sqs.us-east-1.amazonaws.com/123456789012/spot-termination",
			region:    "us-east-1",
			wantError: true,
		},
		{
			name:      "region mismatch",
			queueURL:  validQueueURL,
			region:    "us-west-2",
			wantError: true,
		},
		{
			name:      "region is required when queue is set",
			queueURL:  validQueueURL,
			wantError: true,
		},
		{
			name:             "queue URL must be unique across clusters",
			queueURL:         validQueueURL,
			region:           "us-east-1",
			existingQueueURL: []string{validQueueURL},
			wantError:        true,
		},
		{
			name:      "queue URL has a maximum length",
			queueURL:  validQueueURL[:strings.LastIndex(validQueueURL, "/")+1] + strings.Repeat("a", maxTerminationHandlerQueueURL),
			region:    "us-east-1",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateSpotTerminationHandlerQueueURL(tt.queueURL, tt.region, tt.existingQueueURL)
			if gotError := len(errs) != 0; gotError != tt.wantError {
				t.Fatalf("ValidateSpotTerminationHandlerQueueURL() errors = %v, wantError %t", errs, tt.wantError)
			}
			for _, err := range errs {
				if err.Field != terminationHandlerQueueURLField {
					t.Errorf("error field = %q, want %q", err.Field, terminationHandlerQueueURLField)
				}
			}
		})
	}
}

func TestSpotTerminationHandlerQueueAlreadyInUse(t *testing.T) {
	errs := SpotTerminationHandlerQueueAlreadyInUse()
	if len(errs) != 1 {
		t.Fatalf("error count = %d, want 1", len(errs))
	}
	if errs[0].Field != terminationHandlerQueueURLField {
		t.Errorf("error field = %q, want %q", errs[0].Field, terminationHandlerQueueURLField)
	}
	if !strings.Contains(errs[0].Reason, "unique across clusters") {
		t.Errorf("error reason = %q, want queue uniqueness message", errs[0].Reason)
	}
}
