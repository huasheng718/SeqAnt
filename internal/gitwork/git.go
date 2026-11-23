package gitwork

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"agentflow/internal/procwatch"
)

// Run executes git with the context token applied through AuthConfigArgs and
// Env. Output is trimmed and redacted; on failure it is part of the error.
func Run(ctx context.Context, workspace string, args ...string) (string, error) {
	token := TokenFrom(ctx)
	sshEnv, cleanup, err := sshCommandEnvironment(ctx, args)
	if err != nil {
		return "", err
	}
	defer cleanup()
	out, err := command(ctx, workspace, token, sshEnv, args).CombinedOutput()
	text := Redact(strings.TrimSpace(string(out)), contextTokens(ctx)...)
	if err != nil {
		if text != "" {
			return text, fmt.Errorf("%w: %s", err, text)
		}
		return text, err
	}
	return text, nil
}

// Settings that bound a clone, read on whichever host runs it (the platform or
// an execution machine). See docs/deployment.md.
const (
	CloneStallEnv   = "AGENTFLOW_GIT_CLONE_STALL_TIMEOUT"
	CloneTimeoutEnv = "AGENTFLOW_GIT_CLONE_TIMEOUT"
)

// Watch runs a long git command (clone) like Run, but through procwatch:
// output is streamed to the context's observer, a command that stops printing
// is killed, and the error carries only the last lines of output.
func Watch(ctx context.Context, workspace string, opts procwatch.Options, args ...string) (string, error) {
	token := TokenFrom(ctx)
	sshEnv, cleanup, err := sshCommandEnvironment(ctx, args)
	if err != nil {
		return "", err
	}
	defer cleanup()
	opts.Redact = func(line string) string { return Redact(line, contextTokens(ctx)...) }
	text, err := procwatch.Run(ctx, func(ctx context.Context) *exec.Cmd { return command(ctx, workspace, token, sshEnv, args) }, opts)
	if err != nil && text != "" {
		return text, fmt.Errorf("%w: %s", err, text)
	}
	return text, err
}

func command(ctx context.Context, workspace, token string, sshEnv map[string]string, args []string) *exec.Cmd {
	dir := CommandDir(workspace)
	hostTokens := HostTokensFrom(ctx)
	if len(sshEnv) > 0 {
		token = ""
		hostTokens = nil
	}
	hostEnv := SandboxEnv(token)
	firstHostConfig, _ := strconv.Atoi(hostEnv["GIT_CONFIG_COUNT"])
	BindHostCredentials(hostEnv, hostTokens)
	authArgs := AuthConfigArgs(token)
	configCount, _ := strconv.Atoi(hostEnv["GIT_CONFIG_COUNT"])
	// Command-line -c wins over GIT_CONFIG_COUNT; append the host helpers
	// after the legacy global helper so its reset cannot discard them.
	for i := firstHostConfig; i < configCount; i++ {
		authArgs = append(authArgs, "-c", hostEnv[fmt.Sprintf("GIT_CONFIG_KEY_%d", i)]+"="+hostEnv[fmt.Sprintf("GIT_CONFIG_VALUE_%d", i)])
	}
	cmd := exec.CommandContext(ctx, "git", append(authArgs, args...)...)
	cmd.Dir = dir
	cmd.Env = WithEnvValue(Env(token), "PWD", dir)
	for key, value := range hostEnv {
		cmd.Env = WithEnvValue(cmd.Env, key, value)
	}
	if len(sshEnv) > 0 {
		// Do not let URL rewrites switch a managed SSH operation to another
		// transport. SSH itself only connects to the requested pinned host.
		cmd.Env = WithEnvValue(cmd.Env, "GIT_ALLOW_PROTOCOL", "ssh")
		cmd.Env = WithEnvValue(cmd.Env, "GIT_CONFIG_GLOBAL", os.DevNull)
		cmd.Env = WithEnvValue(cmd.Env, "GIT_CONFIG_NOSYSTEM", "1")
		cmd.Env = WithEnvValue(cmd.Env, "GIT_CONFIG_PARAMETERS", "")
		for k, v := range sshEnv {
			cmd.Env = WithEnvValue(cmd.Env, k, v)
		}
	}
	// A run without a checkout must not discover the platform's own .git
	// when the operator keeps workspaces below a source checkout.
	if absolute, err := filepath.Abs(dir); err == nil {
		cmd.Env = WithEnvValue(cmd.Env, "GIT_CEILING_DIRECTORIES", filepath.Dir(absolute))
	}
	return cmd
}

// C runs `git -C workspace args...`.
func C(ctx context.Context, workspace string, args ...string) (string, error) {
	return Run(ctx, workspace, append([]string{"-C", workspace}, args...)...)
}

// RevParse resolves ref to a commit SHA.
func RevParse(ctx context.Context, workspace, ref string) (string, bool) {
	out, err := C(ctx, workspace, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(out), true
}
