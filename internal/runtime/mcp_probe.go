package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agentflow/internal/domain"
)

// ProbeMCPStdioInSandbox starts a stdio MCP only through the selected sandbox
// driver. It sends initialize, initialized and tools/list, then returns the
// discovered tool names. The API host never starts the configured command.
func ProbeMCPStdioInSandbox(ctx context.Context, manager *Manager, server domain.MCPServerDefinition) ([]string, error) {
	if manager == nil {
		return nil, fmt.Errorf("stdio MCP sandbox manager unavailable")
	}
	if strings.TrimSpace(server.Command) == "" {
		return nil, fmt.Errorf("stdio MCP command is required")
	}
	id := "mcp-check-" + safeID(server.ID)
	// The probe gets a fixed, disposable runtime image. It never reuses a
	// project Dockerfile or mounts a project checkout, so a catalog command
	// cannot reach unrelated workspace files.
	sandbox, err := manager.AcquireRun(ctx, id, domain.SandboxProfile{Driver: "docker"}, SandboxRequirements{}, RunCredentials{}, SandboxResources{Dockerfile: domain.DockerfileDefinition{Content: stdioProbeDockerfile}})
	if err != nil {
		return nil, fmt.Errorf("stdio MCP sandbox unavailable: %w", err)
	}
	defer func() { _ = sandbox.Close(context.Background()); _ = manager.DestroyRun(id) }()
	script := stdioProbeScript(server)
	out, err := sandbox.Exec(ctx, CommandSpec{Argv: []string{"sh", "-lc", script}}, nil)
	if err != nil {
		return nil, fmt.Errorf("stdio MCP probe: %w", err)
	}
	var response struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	scanner := bufio.NewScanner(strings.NewReader(out))
	scanner.Buffer(make([]byte, 64<<10), 2<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var envelope struct {
			ID     int64           `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(line), &envelope) != nil || envelope.ID != 2 {
			continue
		}
		if envelope.Error != nil {
			return nil, fmt.Errorf("stdio MCP tools/list: %s", envelope.Error.Message)
		}
		if err := json.Unmarshal(envelope.Result, &response.Result); err != nil {
			return nil, fmt.Errorf("stdio MCP tools/list result: %w", err)
		}
		tools := make([]string, 0, len(response.Result.Tools))
		for _, tool := range response.Result.Tools {
			if strings.TrimSpace(tool.Name) != "" {
				tools = append(tools, tool.Name)
			}
		}
		return tools, nil
	}
	return nil, fmt.Errorf("stdio MCP probe returned no tools/list response")
}

const stdioProbeDockerfile = "FROM node:22-bookworm-slim\nWORKDIR /workspace\n"

func stdioProbeScript(server domain.MCPServerDefinition) string {
	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"agentflow-check","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
	}
	quoted := make([]string, 0, len(server.Args)+1)
	quoted = append(quoted, shellQuote(server.Command))
	for _, arg := range server.Args {
		quoted = append(quoted, shellQuote(arg))
	}
	lines := make([]string, 0, len(requests))
	for _, req := range requests {
		lines = append(lines, "printf '%s\\n' "+shellQuote(req))
	}
	return "{ " + strings.Join(lines, "; ") + "; } | " + strings.Join(quoted, " ")
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
