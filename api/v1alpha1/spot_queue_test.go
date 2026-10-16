package v1alpha1

import "testing"

func TestSpotTerminationQueueIndexName(t *testing.T) {
	const accountID = "123456789012"
	const queueURL = "https://sqs.us-east-1.amazonaws.com/123456789012/spot-termination"

	name := SpotTerminationQueueIndexName(accountID, queueURL)
	if name == "" || len(name) != 64 {
		t.Fatalf("index name = %q, want a 64-character SHA-256 hex digest", name)
	}
	if got := SpotTerminationQueueIndexName(accountID, queueURL); got != name {
		t.Errorf("same account and queue produced a different index name: %q", got)
	}
	if got := SpotTerminationQueueIndexName("different-account", queueURL); got == name {
		t.Error("different accounts must produce different index names")
	}
	if got := SpotTerminationQueueIndexName(accountID, queueURL+"-different"); got == name {
		t.Error("different queue URLs must produce different index names")
	}
}
