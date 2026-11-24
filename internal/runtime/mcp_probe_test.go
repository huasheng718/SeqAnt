package runtime

import (
	"context"
	"strings"
	"testing"

	"agentflow/internal/domain"
)

func TestStdioProbeScriptQuotesCommandAndArguments(t *testing.T) {
	got := stdioProbeScript(domain.MCPServerDefinition{Command: "server", Args: []string{"--name", "a'b"}})
	if !strings.Contains(got, "'a'\\''b'") || !strings.Contains(got, "tools/list") {
		t.Fatalf("script=%s", got)
	}
}

func TestStdioProbeRequiresSandboxManager(t *testing.T) {
	if _, err := ProbeMCPStdioInSandbox(context.Background(), nil, domain.MCPServerDefinition{ID: "x", Command: "server"}); err == nil {
		t.Fatal("expected unavailable sandbox")
	}
}
