package runner

import (
	"testing"

	"agentflow/internal/domain"
)

func TestInvocationSeparatesPlatformTokensFromGitHubMCP(t *testing.T) {
	inv := &domain.RunnerInvocation{Repositories: []domain.RunnerRepository{
		{CloneURL: "https://gitee.com/team/repo.git", GitToken: "oauth2:gitee-secret"},
		{CloneURL: "https://gitlab.com/team/repo.git", GitToken: "oauth2:gitlab-secret"},
	}}
	if invocationGitToken(inv) != "" {
		t.Fatal("a provider token was passed to GitHub MCP")
	}
	tokens := invocationGitHostTokens(inv)
	if tokens["gitee.com"] != "oauth2:gitee-secret" || tokens["gitlab.com"] != "oauth2:gitlab-secret" {
		t.Fatal("runner lost provider tokens")
	}
	inv.Repositories = append(inv.Repositories, domain.RunnerRepository{CloneURL: "https://github.com/team/repo.git", GitToken: "github-secret"})
	if invocationGitToken(inv) != "github-secret" {
		t.Fatal("GitHub credential was not preserved")
	}
}
