package engine

import (
	"testing"

	"agentflow/internal/domain"
)

func TestRunnerRepositoriesKeepPlatformCredentialsAndPolicy(t *testing.T) {
	run := domain.Run{Repositories: []domain.RepositoryContext{
		{RepositoryID: "github", CloneURL: "https://github.com/team/repo.git"},
		{RepositoryID: "gitee", CloneURL: "https://gitee.com/team/repo.git"},
		{RepositoryID: "gitlab", CloneURL: "https://gitlab.com/team/repo.git"},
		{RepositoryID: "ssh", CloneURL: "git@gitee.com:team/repo.git", Provider: domain.RepositoryProviderSSH},
	}}
	tokens := map[string]string{"gitee.com": "oauth2:gitee-secret", "gitlab.com": "oauth2:gitlab-secret"}
	run.ExecutionContext.DeliveryPolicy = domain.DeliveryPolicyLegacy
	for _, controlled := range []bool{false, true} {
		if controlled {
			run.ExecutionContext.DeliveryPolicy = domain.DeliveryPolicyControlledPR
		}
		invocation := &domain.RunnerInvocation{}
		bindRunnerRepositories(invocation, run, "github-secret", "owner", "owner@example.invalid", tokens)
		want := []string{"github-secret", "oauth2:gitee-secret", "oauth2:gitlab-secret", ""}
		if len(invocation.Repositories) != len(want) {
			t.Fatal("not all repositories were bound")
		}
		for i, repository := range invocation.Repositories {
			token := want[i]
			if controlled {
				token = ""
			}
			if repository.GitToken != token {
				t.Fatalf("wrong credential for %s (controlled=%v)", repository.RepositoryID, controlled)
			}
		}
	}
}
