// internal/stages/signoff/signoff.go
package signoff

import (
	"log"
	"strings"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"

	"github.com/captain-corgi/go-adk-example/internal/keys"
)

// New builds the sign-off gate. With autoApprove it copies plan_output to
// signoff_output. Otherwise it pauses for a human approve/edit decision via
// workflow HITL (verified manually; the unit test covers the AUTO_APPROVE path
// used by the E2E run).
func New(autoApprove bool) (agent.Agent, error) {
	rerun := true
	node := workflow.NewEmittingFunctionNode[any, any]("signoff",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (any, error) {
			approved := stateString(ctx, keys.Plan)

			if !autoApprove {
				reply, err := workflow.ResumeOrRequestInput(ctx, emit, session.RequestInput{
					InterruptID: "signoff",
					Message:     "Approve this campaign plan? Reply 'approve' or paste an edited plan.",
				})
				if err != nil {
					return nil, err
				}
				decision, _ := reply.(string)
				switch classifyReply(decision) {
				case signoffEdit:
					approved = decision // user pasted an edited plan
				case signoffReject:
					// No pipeline-abort path exists in Phase 0, so a rejection
					// keeps the synthesis plan rather than overwriting it with
					// the rejection word. A real reject/abort is Phase 1 HITL.
					log.Printf("signoff: rejection %q received; no abort path in Phase 0, keeping synthesis plan", strings.TrimSpace(decision))
				}
			}

			// Publish signoff_output through the event stream. ctx.State().Set is
			// unobservable from an EmittingFunctionNode (runEmitting discards
			// ctx.actions.StateDelta and does not special-case a *session.Event
			// return), so emit the StateDelta event directly and return nil to
			// suppress the terminal event. keys.Plan is also overwritten with the
			// approved/edited plan so the downstream drafter (templates
			// {plan_output}) and finalize (reads keys.Plan) build from what was
			// actually signed off, not the synthesis draft.
			ev := session.NewEvent(ctx, ctx.InvocationID())
			ev.Actions = session.EventActions{StateDelta: map[string]any{
				keys.Signoff: approved,
				keys.Plan:    approved,
			}}
			if err := emit(ev); err != nil {
				return nil, err
			}
			return nil, nil
		}, workflow.NodeConfig{RerunOnResume: &rerun})

	eb := workflow.NewEdgeBuilder()
	eb.Add(workflow.Start, node)
	return workflowagent.New(workflowagent.Config{
		Name:        "signoff",
		Description: "Human sign-off gate between synthesis and build.",
		Edges:       eb.Build(),
	})
}

// signoffAction is the outcome of classifying a human reply at the sign-off gate.
type signoffAction int

const (
	signoffApprove signoffAction = iota // explicit "approve" or empty: keep the synthesis plan
	signoffReject                       // explicit rejection word: keep the synthesis plan (no abort in Phase 0)
	signoffEdit                         // anything else: treat the reply as an edited plan
)

// classifyReply interprets a sign-off reply. The Phase-0 prompt offers only
// "approve" or "paste an edited plan"; an explicit rejection (or an empty
// reply) keeps the existing synthesis plan instead of overwriting it with the
// reply text, so answering "no"/"reject" can never become the campaign plan.
func classifyReply(reply string) signoffAction {
	t := strings.TrimSpace(reply)
	if t == "" || strings.EqualFold(t, "approve") {
		return signoffApprove
	}
	if isRejection(t) {
		return signoffReject
	}
	return signoffEdit
}

// isRejection reports whether s is an unambiguous rejection of the plan.
func isRejection(s string) bool {
	switch strings.ToLower(s) {
	case "no", "n", "nope", "nah", "reject", "rejected", "deny", "denied",
		"decline", "declined", "cancel", "cancelled", "canceled",
		"stop", "abort", "false":
		return true
	}
	return false
}

func stateString(ctx agent.Context, key string) string {
	v, _ := ctx.State().Get(key)
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
