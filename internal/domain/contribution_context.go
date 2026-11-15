package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// RunnerContributionContextSHA identifies the frozen execution input, before
// claim/lease fields exist. It is a P0 assignment manifest, not a TeamAI or
// governed-memory version. Provider/Git credential values and the hash itself
// do not enter the digest. Attempt identity is bound separately by the lease.
func RunnerContributionContextSHA(task RunnerTask) (string, error) {
	public := PublicRunnerTask(task)
	if public.Invocation != nil && public.Invocation.WorkspaceIsolation != nil {
		isolation := *public.Invocation.WorkspaceIsolation
		isolation.ContextSHA256 = ""
		public.Invocation.WorkspaceIsolation = &isolation
	}
	manifest := struct {
		Version           int                `json:"version"`
		TaskID            string             `json:"taskId"`
		RunID             string             `json:"runId"`
		NodeID            string             `json:"nodeId"`
		DeliveryPolicy    string             `json:"deliveryPolicy"`
		Provider          string             `json:"provider"`
		Prompt            string             `json:"prompt"`
		Args              []string           `json:"args"`
		Command           string             `json:"command"`
		Invocation        *RunnerInvocation  `json:"invocation"`
		Sandbox           *RunnerSandboxSpec `json:"sandbox"`
		ExecutionDeadline *time.Time         `json:"executionDeadline,omitempty"`
	}{1, public.ID, public.RunID, public.NodeID, public.DeliveryPolicy, public.Provider, public.Prompt, public.Args, public.Command, public.Invocation, public.Sandbox, public.ExecutionDeadline}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}
