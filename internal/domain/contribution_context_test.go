package domain

import "testing"

func TestContributionContextBindsAssignmentWithoutCredentialsOrLease(t *testing.T) {
	task := RunnerTask{ID: "task", RunID: "run", NodeID: "node", Provider: "codex", DeliveryPolicy: DeliveryPolicyControlledPR, Invocation: &RunnerInvocation{RunID: "run", NodeID: "node", WorkspaceIsolation: &RunnerWorkspaceIsolation{Version: 1}, CodexAPIKey: "first", Repositories: []RunnerRepository{{RepositoryID: "repo", BaseSHA: "1111111111111111111111111111111111111111", GitToken: "first"}}, Inputs: []RunnerInputFile{{Path: "source.txt", Content: "frozen input"}}}}
	first, err := RunnerContributionContextSHA(task)
	if err != nil {
		t.Fatal(err)
	}
	task.Invocation.WorkspaceIsolation.ContextSHA256 = first
	task.Invocation.CodexAPIKey = "rotated"
	task.Invocation.Repositories[0].GitToken = "rotated"
	task.Attempt = 2
	task.LeaseOwner = "runner"
	task.Status = RunnerTaskRunning
	same, err := RunnerContributionContextSHA(task)
	if err != nil || first != same {
		t.Fatal("credential/lease/hash changed immutable context")
	}
	task.Invocation.Inputs[0].Content = "changed input"
	different, err := RunnerContributionContextSHA(task)
	if err != nil || different == first {
		t.Fatal("input change did not invalidate context")
	}
	task.Invocation.Inputs[0].Content = "frozen input"
	task.Invocation.Repositories[0].BaseSHA = "2222222222222222222222222222222222222222"
	different, err = RunnerContributionContextSHA(task)
	if err != nil || different == first {
		t.Fatal("base change did not invalidate context")
	}
	if task.Invocation.WorkspaceIsolation.ContextSHA256 != first || task.Invocation.CodexAPIKey != "rotated" || task.Invocation.Repositories[0].GitToken != "rotated" {
		t.Fatal("digest mutated original assignment")
	}
}
