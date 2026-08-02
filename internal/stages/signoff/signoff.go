// internal/stages/signoff/signoff.go
package signoff

import (
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
				if trimmed := strings.TrimSpace(decision); trimmed != "" &&
					!strings.EqualFold(trimmed, "approve") {
					approved = decision // edited plan
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

func stateString(ctx agent.Context, key string) string {
	v, _ := ctx.State().Get(key)
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
