package repo

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"agentflow/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrRunnerContributionInvalid  = errors.New("invalid runner contribution")
	ErrRunnerContributionConflict = errors.New("runner contribution conflicts with an existing receipt")
	ErrRunnerContributionTooLarge = errors.New("runner contribution exceeds the 16 MiB limit")
)

// RunnersRepo owns these bytes in the same platform database. Preview expiry
// and preview garbage collection never own contribution blobs.
type runnerContributionBlobRow struct {
	SHA256  string    `gorm:"column:sha256;primaryKey;size:64"`
	Content []byte    `gorm:"column:content;type:longblob"`
	Size    int64     `gorm:"column:size;not null"`
	Created time.Time `gorm:"column:created_at;not null"`
}

func (runnerContributionBlobRow) TableName() string { return "runner_contribution_blobs" }

func validContributionSHA256(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func invalidContribution(reason string) error {
	return fmt.Errorf("%w: %s", ErrRunnerContributionInvalid, reason)
}

func contributionSpec(task domain.RunnerTask, repositoryID string) (domain.RunnerContributionSpec, error) {
	if task.Action != "" || task.RunID == "" || task.NodeID == "" || task.Invocation == nil {
		return domain.RunnerContributionSpec{}, invalidContribution("export requires a workflow execution task")
	}
	repositories := task.Invocation.Repositories
	if len(repositories) == 0 && task.Invocation.Repository != nil {
		repositories = []domain.RunnerRepository{*task.Invocation.Repository}
	}
	seen := make(map[string]bool, len(task.ContributionSpecs))
	var selected domain.RunnerContributionSpec
	for _, spec := range task.ContributionSpecs {
		if spec.RepositoryID == "" || strings.TrimSpace(spec.RepositoryID) != spec.RepositoryID || seen[spec.RepositoryID] || !domain.ValidWorkspaceBaseSHA(spec.BaseSHA) || !validContributionSHA256(spec.ContextSHA256) || (spec.Dir != "" && !domain.SafeWorkspaceRelPath(spec.Dir)) {
			return selected, invalidContribution("invalid or duplicate frozen repository spec")
		}
		seen[spec.RepositoryID] = true
		if spec.MaxBytes <= 0 || spec.MaxBytes > domain.RunnerContributionMaxBytes {
			return selected, ErrRunnerContributionTooLarge
		}
		matches := 0
		for _, repository := range repositories {
			if repository.RepositoryID == spec.RepositoryID && repository.Dir == spec.Dir && repository.BaseSHA == spec.BaseSHA {
				matches++
			}
		}
		if matches != 1 {
			return selected, invalidContribution("frozen spec does not match exactly one invocation repository")
		}
		if spec.RepositoryID == repositoryID {
			selected = spec
		}
	}
	if selected.RepositoryID == "" {
		return selected, invalidContribution("repository was not assigned for export")
	}
	return selected, nil
}

func (r *RunnersRepo) contributionTaskForUpload(tx *gorm.DB, taskID, runnerID string, attempt int, repositoryID string, now time.Time) (domain.RunnerTask, domain.RunnerContributionSpec, error) {
	row, ok := r.takeRunnerTaskForLease(tx, taskID, runnerID, attempt, now)
	if !ok || row.DesiredState == domain.RunnerTaskDesiredCancel {
		return domain.RunnerTask{}, domain.RunnerContributionSpec{}, ErrRunnerTaskLeaseLost
	}
	task := runnerTaskFromRow(row)
	spec, err := contributionSpec(task, repositoryID)
	return task, spec, err
}

// ValidateRunnerContributionUpload authorizes receipt without holding a task
// lock while a potentially slow request body streams. Stage rechecks it.
func (r *RunnersRepo) ValidateRunnerContributionUpload(taskID, runnerID string, attempt int, repositoryID string, now time.Time) (domain.RunnerContributionSpec, error) {
	var spec domain.RunnerContributionSpec
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		_, spec, err = r.contributionTaskForUpload(tx, taskID, runnerID, attempt, repositoryID, now)
		return err
	})
	return spec, err
}

// PutRunnerContributionBlob verifies bounded content-addressed bytes. It never
// accepts them as task output; a rejected transfer may leave an orphan blob.
func (r *RunnersRepo) PutRunnerContributionBlob(sha256Hex string, content []byte) error {
	if int64(len(content)) > domain.RunnerContributionMaxBytes {
		return ErrRunnerContributionTooLarge
	}
	if len(content) == 0 || !validContributionSHA256(sha256Hex) {
		return invalidContribution("missing content or invalid SHA256")
	}
	sum := sha256.Sum256(content)
	if hex.EncodeToString(sum[:]) != sha256Hex {
		return invalidContribution("content SHA256 mismatch")
	}
	row := runnerContributionBlobRow{SHA256: sha256Hex, Content: content, Size: int64(len(content)), Created: time.Now().UTC()}
	return r.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

func (r *RunnersRepo) RunnerContributionBlob(sha256Hex string) ([]byte, int64, bool) {
	if !validContributionSHA256(sha256Hex) {
		return nil, 0, false
	}
	var row runnerContributionBlobRow
	if err := r.DB.Where("sha256 = ?", sha256Hex).Take(&row).Error; err != nil {
		return nil, 0, false
	}
	return row.Content, row.Size, true
}

func sameContribution(a, b domain.RunnerContributionReceipt) bool {
	return a.TaskID == b.TaskID && a.Attempt == b.Attempt && a.RepositoryID == b.RepositoryID && a.Dir == b.Dir && a.BaseSHA == b.BaseSHA && a.ResultSHA == b.ResultSHA && a.ContextSHA256 == b.ContextSHA256 && a.SHA256 == b.SHA256 && a.Size == b.Size
}

func validateContributionReceipt(receipt domain.RunnerContributionReceipt, spec domain.RunnerContributionSpec) error {
	if receipt.RepositoryID != spec.RepositoryID || receipt.Dir != spec.Dir || receipt.BaseSHA != spec.BaseSHA || receipt.ContextSHA256 != spec.ContextSHA256 || !domain.ValidWorkspaceBaseSHA(receipt.ResultSHA) || !validContributionSHA256(receipt.SHA256) || receipt.Size <= 0 {
		return invalidContribution("receipt does not match frozen repository/context or complete commit identity")
	}
	if receipt.Size > spec.MaxBytes || receipt.Size > domain.RunnerContributionMaxBytes {
		return ErrRunnerContributionTooLarge
	}
	return nil
}

func contributionBlobExists(tx *gorm.DB, receipt domain.RunnerContributionReceipt) error {
	var row runnerContributionBlobRow
	if err := tx.Select("sha256", "size").Where("sha256 = ? AND size = ?", receipt.SHA256, receipt.Size).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return invalidContribution("durable bundle bytes are missing")
		}
		return err
	}
	return nil
}

// StageRunnerContribution atomically binds a durable blob to the still-live
// attempt. IDs and timestamps are server-owned; identical replay returns the
// original receipt while conflicting replay cannot replace it.
func (r *RunnersRepo) StageRunnerContribution(receipt domain.RunnerContributionReceipt, runnerID string, now time.Time) (domain.RunnerContributionReceipt, error) {
	var staged domain.RunnerContributionReceipt
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		task, spec, err := r.contributionTaskForUpload(tx, receipt.TaskID, runnerID, receipt.Attempt, receipt.RepositoryID, now)
		if err != nil {
			return err
		}
		if err := validateContributionReceipt(receipt, spec); err != nil {
			return err
		}
		if err := contributionBlobExists(tx, receipt); err != nil {
			return err
		}
		for _, pending := range task.PendingContributions {
			if pending.Attempt == receipt.Attempt && pending.RepositoryID == receipt.RepositoryID {
				if !sameContribution(pending, receipt) {
					return ErrRunnerContributionConflict
				}
				staged = pending
				return nil
			}
		}
		receipt.ID = "rc-" + rand.Text()
		receipt.CreatedAt, receipt.AcceptedAt = now.UTC(), time.Time{}
		receipt.StorageKey = receipt.SHA256
		task.PendingContributions = append(task.PendingContributions, receipt)
		task.UpdatedAt = now
		if err := tx.Model(&runnerTaskRow{}).Where("id = ?", task.ID).Update("payload", runnerTaskPayloadBytes(task)).Error; err != nil {
			return err
		}
		staged = receipt
		return nil
	})
	return staged, err
}

// acceptRunnerContributions runs only while CompleteRunnerTask holds the live
// task lock; accepted metadata and terminal success use the same transaction.
func acceptRunnerContributions(tx *gorm.DB, task *domain.RunnerTask, outputs *domain.RunnerTaskOutputs, now time.Time) error {
	var ids []string
	if outputs != nil {
		ids = outputs.ContributionReceipts
	}
	if len(ids) != len(task.ContributionSpecs) {
		return invalidContribution("successful completion requires one server receipt per assigned repository")
	}
	accepted := make([]domain.RunnerContributionReceipt, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		var receipt domain.RunnerContributionReceipt
		for _, pending := range task.PendingContributions {
			if pending.ID == id && pending.TaskID == task.ID && pending.Attempt == task.Attempt {
				receipt = pending
				break
			}
		}
		if receipt.ID == "" || seen[receipt.RepositoryID] {
			return invalidContribution("unknown, duplicate or previous-attempt receipt")
		}
		seen[receipt.RepositoryID] = true
		spec, err := contributionSpec(*task, receipt.RepositoryID)
		if err != nil {
			return err
		}
		if err := validateContributionReceipt(receipt, spec); err != nil {
			return err
		}
		if err := contributionBlobExists(tx, receipt); err != nil {
			return err
		}
		receipt.AcceptedAt = now.UTC()
		receipt.StorageKey = receipt.SHA256
		accepted = append(accepted, receipt)
	}
	task.AcceptedContributions = accepted
	return nil
}
