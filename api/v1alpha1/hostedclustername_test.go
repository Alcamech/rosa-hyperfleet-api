package v1alpha1

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestHostedClusterNameFromDNSReservation(t *testing.T) {
	tests := []struct {
		name        string
		reservation *DNSReservation
		want        string
	}{
		{
			name: "UUID is compacted and truncated",
			reservation: &DNSReservation{ObjectMeta: metav1.ObjectMeta{
				UID: types.UID("550e8400-e29b-41d4-a716-446655440000"),
			}},
			want: "hc-550e8400e29b41d",
		},
		{
			name: "nil reservation",
		},
		{
			name:        "reservation without UID",
			reservation: &DNSReservation{},
		},
		{
			name: "UID without DNS label characters",
			reservation: &DNSReservation{ObjectMeta: metav1.ObjectMeta{
				UID: types.UID("---"),
			}},
		},
		{
			name: "long UID is truncated to the management namespace budget",
			reservation: &DNSReservation{ObjectMeta: metav1.ObjectMeta{
				UID: types.UID(strings.Repeat("a", 45)),
			}},
			want: "hc-" + strings.Repeat("a", 15),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HostedClusterNameFromDNSReservation(tt.reservation); got != tt.want {
				t.Errorf("HostedClusterNameFromDNSReservation(%v) = %q, want %q", tt.reservation, got, tt.want)
			}
		})
	}
}
