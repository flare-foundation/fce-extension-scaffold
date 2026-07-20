package fccutils

import (
	"errors"
	"strings"
	"testing"
)

func TestPolicyInSync(t *testing.T) {
	cases := []struct {
		name    string
		onchain uint64
		proxy   uint64
		want    bool
	}{
		{"exact match", 100, 100, true},
		{"proxy one ahead (rollover window)", 100, 101, true},
		{"proxy one behind", 100, 99, false},
		{"proxy two ahead", 100, 102, false},
		{"genesis", 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := policyInSync(tc.onchain, tc.proxy); got != tc.want {
				t.Fatalf("policyInSync(%d, %d) = %v, want %v", tc.onchain, tc.proxy, got, tc.want)
			}
		})
	}
}

func TestOutOfSyncError(t *testing.T) {
	err := outOfSyncError(1236, 1234) // proxy behind
	if !errors.Is(err, ErrPolicyOutOfSync) {
		t.Fatalf("expected error to wrap ErrPolicyOutOfSync, got %v", err)
	}
	msg := err.Error()
	for _, want := range []string{"1234", "1236", "behind", "404"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message %q missing %q", msg, want)
		}
	}

	ahead := outOfSyncError(1236, 1239) // proxy ahead
	if !strings.Contains(ahead.Error(), "ahead") {
		t.Errorf("expected 'ahead' in %q", ahead.Error())
	}
}
