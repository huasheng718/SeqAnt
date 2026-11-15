package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *runsHandlers) integrateContributions(c *gin.Context) {
	if authRequired() && !h.isPlatformAdmin(c.Request) {
		c.JSON(http.StatusForbidden, gin.H{"error": "controller integration requires platform admin"})
		return
	}
	var input struct {
		RepositoryID string   `json:"repositoryId"`
		TaskIDs      []string `json:"taskIds"`
	}
	if !decode(c, &input) {
		return
	}
	result, err := h.engine.IntegrateRunnerContributions(c.Request.Context(), c.Param("id"), input.RepositoryID, input.TaskIDs)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *runsHandlers) integrationContent(c *gin.Context) {
	if authRequired() && !h.isPlatformAdmin(c.Request) {
		c.JSON(http.StatusForbidden, gin.H{"error": "controller integration requires platform admin"})
		return
	}
	value, ok := h.repo.ContributionIntegration(c.Param("integrationId"))
	if !ok || value.RunID != c.Param("id") {
		c.JSON(http.StatusNotFound, gin.H{"error": "integration not found"})
		return
	}
	if err := h.engine.ValidateContributionIntegration(value); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	raw, size, ok := h.repo.RunnerContributionBlob(value.BundleSHA256)
	sum := sha256.Sum256(raw)
	if !ok || size != value.BundleSize || hex.EncodeToString(sum[:]) != value.BundleSHA256 {
		c.JSON(http.StatusConflict, gin.H{"error": "integration bytes unavailable"})
		return
	}
	c.Header("X-Contribution-Result-SHA", value.ResultSHA)
	c.Header("X-Contribution-SHA256", value.BundleSHA256)
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "application/x-git-bundle", raw)
}
