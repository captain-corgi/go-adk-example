// internal/stages/signoff/signoff_classify_test.go
package signoff

import "testing"

// TestClassifyReply pins the sign-off reply classification without needing the
// HITL harness: approval/empty keep the synthesis plan, an explicit rejection
// word keeps it too (Phase 0 has no abort path), and anything else is treated
// as an edited plan. The rejection carve-out exists so answering "no"/"reject"
// can never become the campaign plan.
func TestClassifyReply(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want signoffAction
	}{
		{"empty", "", signoffApprove},
		{"whitespace", "   \n ", signoffApprove},
		{"approve lower", "approve", signoffApprove},
		{"approve mixed case", "  Approve\n", signoffApprove},
		{"approve upper", "APPROVE", signoffApprove},

		{"reject no", "no", signoffReject},
		{"reject Reject", "Reject", signoffReject},
		{"reject cancel", "cancel", signoffReject},
		{"reject decline trimmed", "  decline ", signoffReject},
		{"reject abort", "abort", signoffReject},

		{"edit short instruction", "make the CTA bolder", signoffEdit},
		{"edit json", `{"hook":"x","offer":"y"}`, signoffEdit},
		{"edit prose", "Lead with the free tier and use a playful tone.", signoffEdit},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := classifyReply(c.in); got != c.want {
				t.Errorf("classifyReply(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
