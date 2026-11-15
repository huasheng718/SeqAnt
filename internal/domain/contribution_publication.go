package domain

import "time"

// ContributionGateEvidence is controller-owned evidence for one immutable
// integration. Runner success/output is never a gate publication capability.
type ContributionGateEvidence struct {
	GateRunID       string    `json:"gateRunId,omitempty"`
	GateNodeID      string    `json:"gateNodeId,omitempty"`
	GateTaskID      string    `json:"gateTaskId,omitempty"`
	GateTaskAttempt int       `json:"gateTaskAttempt,omitempty"`
	ArtifactID      string    `json:"artifactId,omitempty"`
	ID              string    `json:"id"`
	Kind            string    `json:"kind"` // fast, slow, review
	IntegrationID   string    `json:"integrationId"`
	ResultSHA       string    `json:"resultSHA"`
	ContextSHA256   string    `json:"contextSHA256"`
	ManifestSHA256  string    `json:"manifestSHA256"`
	Status          string    `json:"status"` // passed, failed, rejected
	EvidenceRef     string    `json:"evidenceRef"`
	EvidenceSHA256  string    `json:"evidenceSHA256"`
	ReviewerID      string    `json:"reviewerId,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}

// ContributionPublication stops at pr_ready. It grants no merge or business
// completion authority; an external adapter must be strictly create-only.
type ContributionPublication struct {
	ID             string                     `json:"id"`
	RunID          string                     `json:"runId"`
	RepositoryID   string                     `json:"repositoryId"`
	ResultSHA      string                     `json:"resultSHA"`
	ContextSHA256  string                     `json:"contextSHA256"`
	ManifestSHA256 string                     `json:"manifestSHA256"`
	TargetBranch   string                     `json:"targetBranch"`
	Branch         string                     `json:"branch"`
	State          string                     `json:"state"` // gating, ready, publishing, pr_ready, rejected
	Gates          []ContributionGateEvidence `json:"gates"`
	Attempt        int                        `json:"attempt"`
	LeaseOwner     string                     `json:"leaseOwner,omitempty"`
	LeaseUntil     *time.Time                 `json:"leaseUntil,omitempty"`
	PRURL          string                     `json:"prURL,omitempty"`
	PRNumber       int                        `json:"prNumber,omitempty"`
	CreatedAt      time.Time                  `json:"createdAt"`
	UpdatedAt      time.Time                  `json:"updatedAt"`
}
