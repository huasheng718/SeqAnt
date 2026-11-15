package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agentflow/internal/domain"
	"agentflow/internal/gitwork"
)

// UploadContribution transfers only the standalone Git bundle. It cannot
// choose a controller path or make a staged receipt accepted task output.
func (c *Client) UploadContribution(ctx context.Context, taskID string, attempt int, spec domain.RunnerContributionSpec, bundle gitwork.ContributionBundle) (domain.RunnerContributionReceipt, error) {
	var receipt domain.RunnerContributionReceipt
	if attempt <= 0 || taskID == "" || spec.RepositoryID == "" || bundle.Size <= 0 || bundle.Size > domain.RunnerContributionMaxBytes || (spec.MaxBytes > 0 && bundle.Size > spec.MaxBytes) {
		return receipt, errors.New("invalid contribution upload identity or size")
	}
	file, err := os.Open(bundle.Path)
	if err != nil {
		return receipt, err
	}
	defer file.Close()
	path := fmt.Sprintf("/runners/tasks/%s/contributions/%s/content?attempt=%d", url.PathEscape(taskID), url.PathEscape(spec.RepositoryID), attempt)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.base+path, io.LimitReader(file, bundle.Size))
	if err != nil {
		return receipt, err
	}
	req.ContentLength = bundle.Size
	req.Header.Set("Content-Type", "application/x-git-bundle")
	req.Header.Set("Authorization", "Bearer "+c.sessionToken())
	req.Header.Set("X-Contribution-Base-SHA", spec.BaseSHA)
	req.Header.Set("X-Contribution-Result-SHA", bundle.ResultSHA)
	req.Header.Set("X-Contribution-Context-SHA256", spec.ContextSHA256)
	resp, err := c.http.Do(req)
	if err != nil {
		return receipt, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return receipt, ErrUnauthorized
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return receipt, err
	}
	if resp.StatusCode != http.StatusOK {
		var detail struct {
			Error string `json:"error"`
		}
		if resp.StatusCode == http.StatusConflict && json.Unmarshal(payload, &detail) == nil && detail.Error == ErrTaskLeaseLost.Error() {
			return receipt, ErrTaskLeaseLost
		}
		return receipt, fmt.Errorf("contribution upload rejected (%d)", resp.StatusCode)
	}
	if err := json.Unmarshal(payload, &receipt); err != nil {
		return receipt, fmt.Errorf("decode contribution receipt: %w", err)
	}
	if receipt.ID == "" || receipt.TaskID != taskID || receipt.Attempt != attempt || receipt.RepositoryID != spec.RepositoryID || receipt.Dir != spec.Dir || receipt.BaseSHA != spec.BaseSHA || receipt.ResultSHA != bundle.ResultSHA || receipt.ContextSHA256 != spec.ContextSHA256 || receipt.SHA256 != bundle.SHA256 || receipt.Size != bundle.Size || !receipt.AcceptedAt.IsZero() {
		return domain.RunnerContributionReceipt{}, errors.New("contribution receipt identity mismatch")
	}
	return receipt, nil
}

func (d *Daemon) exportTaskContributions(ctx context.Context, task domain.RunnerTask, outputs *domain.RunnerTaskOutputs) (*domain.RunnerTaskOutputs, error) {
	if len(task.ContributionSpecs) == 0 {
		return outputs, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	// Keep bundle files outside the checkout so export cannot commit its own
	// output or expose private temporary data to a later workspace upload.
	dir, err := os.MkdirTemp("", "agentflow-runner-contribution-")
	if err != nil {
		return outputs, err
	}
	defer os.RemoveAll(dir)
	if outputs == nil {
		outputs = &domain.RunnerTaskOutputs{}
	}
	outputs.ContributionReceipts = nil
	for i, spec := range task.ContributionSpecs {
		repository, err := contributionRepositoryPath(d.taskWorkspace(task), spec.Dir)
		if err != nil {
			return outputs, err
		}
		limit := spec.MaxBytes
		if limit <= 0 || limit > domain.RunnerContributionMaxBytes {
			limit = domain.RunnerContributionMaxBytes
		}
		bundle, err := gitwork.ExportContribution(ctx, repository, filepath.Join(dir, fmt.Sprintf("%d.bundle", i)), spec.BaseSHA, fmt.Sprintf("AgentFlow contribution %s attempt %d", task.ID, task.Attempt), limit)
		if err != nil {
			return outputs, fmt.Errorf("export repository %s: %w", spec.RepositoryID, err)
		}
		receipt, err := d.client.UploadContribution(ctx, task.ID, task.Attempt, spec, bundle)
		if err != nil {
			return outputs, fmt.Errorf("upload repository %s: %w", spec.RepositoryID, err)
		}
		outputs.ContributionReceipts = append(outputs.ContributionReceipts, receipt.ID)
	}
	return outputs, nil
}

func contributionRepositoryPath(workspace, dir string) (string, error) {
	if dir != "" && dir != "." && !domain.SafeWorkspaceRelPath(dir) {
		return "", errors.New("unsafe contribution repository directory")
	}
	root, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("contribution repository escapes workspace")
	}
	return target, nil
}
