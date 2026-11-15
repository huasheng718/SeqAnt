package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"agentflow/internal/domain"
	"agentflow/internal/engine"
	"agentflow/internal/repo"
	"agentflow/internal/runtime"
	"agentflow/internal/tracker"
)

func TestManualRunAttemptWorkspaceUsesExplicitRepositoryPins(t *testing.T) {
	s, err := repo.OpenTest()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.PutUser(domain.User{ID: 1, LoginName: "workspace-owner"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutProject(domain.Project{ID: "workspace-project", Key: "WORKSPACE", Name: "Workspace", OwnerID: 1, DefaultRepositoryID: "allowed-repo"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutRepository(domain.Repository{ID: "allowed-repo", ProjectID: "workspace-project", Name: "Allowed", CloneURL: "https://example.test/repo.git", DefaultBranch: "main"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutRunner(domain.Runner{ID: "workspace-runner", OwnerUserID: 1, RunnerVersion: domain.RunnerVersionWorkspaceIsolation}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDefaultRunner(1, "workspace-runner", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := s.PutWorkflow(domain.WorkflowDefinition{ID: "workspace-manual", Name: "Manual", Generic: true, SandboxProfileID: "local", Nodes: []domain.WorkflowNode{{ID: "hold", Name: "Hold", Type: "approval", MaxAttempts: 1}}}); err != nil {
		t.Fatal(err)
	}
	eng := engine.New(s, runtime.NewManager(t.TempDir()))
	eng.ConfigureExecution("workspace-api", "test", time.Minute, false)
	h := New(s, eng, tracker.NewService(), "", "")
	for _, pins := range []map[string]string{nil, {"allowed-repo": "main"}, {"allowed-repo": strings.Repeat("a", 40), "other-repo": strings.Repeat("a", 40)}} {
		requestJSON[map[string]any](t, h, http.MethodPost, "/api/runs", map[string]any{"workflowId": "workspace-manual", "projectId": "workspace-project", "runnerWorkspaceLayout": "attempt", "workspaceBaseSHAs": pins}, http.StatusBadRequest)
	}
	run := requestJSON[domain.Run](t, h, http.MethodPost, "/api/runs", map[string]any{"workflowId": "workspace-manual", "projectId": "workspace-project", "runnerWorkspaceLayout": "attempt", "workspaceBaseSHAs": map[string]string{"allowed-repo": strings.Repeat("a", 40)}}, http.StatusCreated)
	if run.ExecutionContext.RunnerWorkspaceLayout != domain.RunnerWorkspaceLayoutAttempt || run.ExecutionContext.WorkspaceBaseSHAs["allowed-repo"] != strings.Repeat("a", 40) {
		t.Fatalf("manual API did not freeze requested isolation: %+v", run.ExecutionContext)
	}
}
