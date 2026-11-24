package httpapi

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"agentflow/internal/domain"
	"agentflow/internal/mcpclient"
	"agentflow/internal/repo"
	"agentflow/internal/runtime"

	"github.com/gin-gonic/gin"
)

// Snapshot of the bundled MCP market catalog.
//
//go:embed mcp_market_catalog.json
var mcpMarketCatalogJSON []byte

type mcpMarketItem struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Category        string   `json:"category"`
	Tags            []string `json:"tags"`
	IconURL         string   `json:"iconUrl,omitempty"`
	DetailURL       string   `json:"detailUrl,omitempty"`
	URL             string   `json:"url"`
	Transport       string   `json:"transport"`
	AuthType        string   `json:"authType"`
	Official        bool     `json:"official"`
	Aliases         []string `json:"aliases,omitempty"`
	TemplateVersion string   `json:"templateVersion,omitempty"`
	Availability    string   `json:"availability,omitempty"` // ready, candidate
	Requirements    []string `json:"requirements,omitempty"`
}

type mcpMarketCatalogFile struct {
	Source    string          `json:"source"`
	Tag       string          `json:"tag"`
	FetchedAt string          `json:"fetchedAt"`
	Items     []mcpMarketItem `json:"items"`
}

var (
	mcpMarketCatalog mcpMarketCatalogFile
	mcpMarketItems   []mcpMarketItem
)

func init() {
	if err := json.Unmarshal(mcpMarketCatalogJSON, &mcpMarketCatalog); err != nil {
		panic("mcp market catalog: " + err.Error())
	}
	mcpMarketItems = mcpMarketCatalog.Items
}

var mcpInstallIDUnsafe = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func mcpInstallID(marketID, scope, owner string) string {
	clean := func(v string) string { return strings.Trim(mcpInstallIDUnsafe.ReplaceAllString(v, "-"), "-") }
	id := fmt.Sprintf("mcp-%s-%s-%s", clean(marketID), clean(scope), clean(owner))
	if len(id) > 64 {
		// Keep existing IDs stable while bounding compound project/custom IDs
		// to the persisted key size. The digest includes the entire scope/owner.
		sum := sha256.Sum256([]byte(id))
		id = fmt.Sprintf("%.31s-%x", id, sum[:16])
	}
	return id
}

// defaultProjectMarketMCPAliases are installed on every project so new users
// can create Gongfeng MRs and sync TAPD without opening the MCP market.
var defaultProjectMarketMCPAliases = []string{}

// installMarketMCP adds one market catalog item to a project. If the
// same catalog entry is already installed there, it returns the existing row
// and created=false.
func installMarketMCP(st *repo.Store, marketID, projectID string) (domain.MCPServerDefinition, bool, error) {
	return installMarketMCPForOwner(st, marketID, "project", projectID, 0)
}

func installMarketMCPForOwner(st *repo.Store, marketID, scope, projectID string, ownerUserID int64) (domain.MCPServerDefinition, bool, error) {
	item, ok := marketItem(strings.TrimSpace(marketID))
	if !ok {
		return domain.MCPServerDefinition{}, false, fmt.Errorf("MCP 市场条目不存在")
	}
	scope = strings.TrimSpace(scope)
	projectID = strings.TrimSpace(projectID)
	if item.Availability != "" && item.Availability != "ready" {
		return domain.MCPServerDefinition{}, false, fmt.Errorf("该 MCP 尚未完成平台适配")
	}
	if scope == "project" && projectID == "" {
		return domain.MCPServerDefinition{}, false, fmt.Errorf("项目级 MCP 需要当前项目")
	}
	if scope == "personal" && ownerUserID <= 0 {
		return domain.MCPServerDefinition{}, false, fmt.Errorf("个人级 MCP 需要当前用户")
	}
	if scope != "project" && scope != "personal" {
		return domain.MCPServerDefinition{}, false, fmt.Errorf("请选择项目级或个人级")
	}
	if st == nil {
		return domain.MCPServerDefinition{}, false, fmt.Errorf("项目不存在")
	}
	if scope == "project" {
		if _, exists := st.Project(projectID); !exists {
			return domain.MCPServerDefinition{}, false, fmt.Errorf("项目不存在")
		}
	}
	server := domain.MCPServerDefinition{
		ID: mcpInstallID(item.ID, scope, func() string {
			if scope == "personal" {
				return fmt.Sprintf("%d", ownerUserID)
			}
			return projectID
		}()), MarketID: item.ID, Name: item.Name,
		Description: item.Description, IconURL: item.IconURL, Transport: item.Transport,
		URL: item.URL, AuthType: item.AuthType, Scope: scope, ProjectID: projectID, OwnerUserID: ownerUserID,
		TemplateVersion: item.TemplateVersion, TimeoutSec: 60, UpdatedAt: time.Now().UTC(),
	}
	createdServer, created, err := st.CreateMCPServerIfAbsent(server)
	if err != nil {
		return domain.MCPServerDefinition{}, false, err
	}
	if !created && (!sameMarketItem(createdServer.MarketID, item.ID) || createdServer.Scope != scope || createdServer.ProjectID != projectID || createdServer.OwnerUserID != ownerUserID) {
		return domain.MCPServerDefinition{}, false, fmt.Errorf("当前范围已经存在相同安装标识")
	}
	return createdServer, created, nil
}

func ensureDefaultProjectMCPs(st *repo.Store, projectID string) error {
	for _, alias := range defaultProjectMarketMCPAliases {
		if _, _, err := installMarketMCP(st, alias, projectID); err != nil {
			return err
		}
	}
	return nil
}

func ensureDefaultMCPsForAllProjects(st *repo.Store) error {
	if st == nil {
		return nil
	}
	for _, project := range st.Projects() {
		if err := ensureDefaultProjectMCPs(st, project.ID); err != nil {
			return err
		}
	}
	return nil
}

func marketItem(id string) (mcpMarketItem, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return mcpMarketItem{}, false
	}
	for _, item := range mcpMarketItems {
		if item.ID == id || slices.Contains(item.Aliases, id) {
			return item, true
		}
	}
	return mcpMarketItem{}, false
}

// 路由: g.GET("/api/mcp-market", h.mcpMarket)
type marketHandlers struct {
	*deps
}

func sameMarketItem(a, b string) bool {
	itemA, okA := marketItem(a)
	itemB, okB := marketItem(b)
	if okA && okB {
		return itemA.ID == itemB.ID
	}
	return a != "" && a == b
}

func mcpMarketSearchText(item mcpMarketItem) string {
	parts := []string{item.ID, item.Name, item.Description, item.Category, item.URL}
	parts = append(parts, item.Tags...)
	parts = append(parts, item.Aliases...)
	return strings.ToLower(strings.Join(parts, " "))
}

func filterMCPMarket(items []mcpMarketItem, q string) []mcpMarketItem {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		out := make([]mcpMarketItem, len(items))
		copy(out, items)
		return out
	}
	terms := strings.Fields(q)
	out := make([]mcpMarketItem, 0)
	for _, item := range items {
		hay := mcpMarketSearchText(item)
		matched := true
		for _, term := range terms {
			if !strings.Contains(hay, term) {
				matched = false
				break
			}
		}
		if matched {
			out = append(out, item)
		}
	}
	return out
}

func (h *marketHandlers) mcpMarket(c *gin.Context) {
	items := filterMCPMarket(mcpMarketItems, c.Query("q"))
	c.JSON(http.StatusOK, map[string]any{
		"source":    mcpMarketCatalog.Source,
		"fetchedAt": mcpMarketCatalog.FetchedAt,
		"total":     len(mcpMarketItems),
		"items":     items,
	})
}

// mcpMarketCheck is an explicit, read-only HTTP MCP probe. It does not create
// an installation or call any business tool; the result is intentionally
// session-shaped and must not be treated as a permanent health grant.
func (h *marketHandlers) mcpMarketCheck(c *gin.Context) {
	var input struct {
		MarketID        string `json:"marketId"`
		Scope           string `json:"scope"`
		ProjectID       string `json:"projectId"`
		TemplateVersion string `json:"templateVersion"`
		InstallationID  string `json:"installationId"`
	}
	if !decode(c, &input) {
		return
	}
	item, ok := marketItem(input.MarketID)
	if !ok {
		c.JSON(http.StatusNotFound, map[string]string{"error": "MCP 市场条目不存在"})
		return
	}
	if item.Availability != "" && item.Availability != "ready" {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "该 MCP 尚未完成平台适配"})
		return
	}
	if input.TemplateVersion != "" && input.TemplateVersion != item.TemplateVersion {
		c.JSON(http.StatusConflict, map[string]string{"error": "MCP 模板版本已变化，请刷新目录后重试"})
		return
	}
	if strings.TrimSpace(input.Scope) == "project" && !h.requireProjectWriter(c, strings.TrimSpace(input.ProjectID)) {
		return
	}
	if strings.TrimSpace(input.Scope) == "personal" {
		if _, authenticated := h.requestUser(c.Request, true); !authenticated {
			c.JSON(http.StatusUnauthorized, map[string]string{"error": "添加个人 MCP 前请先登录"})
			return
		}
	}
	if item.Transport != "http" {
		if strings.TrimSpace(input.InstallationID) == "" {
			c.JSON(http.StatusOK, map[string]any{"status": "unavailable", "code": "STDIO_SANDBOX_REQUIRED", "testLocation": "sandbox"})
			return
		}
		server, found := h.repo.MCPServer(strings.TrimSpace(input.InstallationID))
		if !found || server.MarketID != item.ID {
			c.JSON(http.StatusNotFound, map[string]string{"error": "公共 MCP 配置不存在"})
			return
		}
		root, err := os.MkdirTemp("", "agentflow-mcp-check-")
		if err != nil {
			c.JSON(http.StatusOK, map[string]any{"status": "unavailable", "code": "SANDBOX_UNAVAILABLE", "testLocation": "sandbox"})
			return
		}
		defer os.RemoveAll(root)
		ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
		defer cancel()
		tools, err := runtime.ProbeMCPStdioInSandbox(ctx, runtime.NewManager(root), server)
		if err != nil {
			c.JSON(http.StatusOK, map[string]any{"status": "unavailable", "code": "STDIO_SANDBOX_UNAVAILABLE", "testLocation": "sandbox"})
			return
		}
		c.JSON(http.StatusOK, map[string]any{"status": "passed", "code": "OK", "testLocation": "sandbox", "testedAt": time.Now().UTC(), "tools": tools})
		return
	}
	if strings.TrimSpace(item.URL) == "" {
		c.JSON(http.StatusOK, map[string]any{"status": "failed", "code": "CONFIG_INVALID", "testLocation": "platform"})
		return
	}
	headers := map[string]string{}
	if item.AuthType == "github-token" {
		user, authenticated := h.requestUser(c.Request, true)
		if !authenticated || user.ID <= 0 {
			c.JSON(http.StatusOK, map[string]any{"status": "failed", "code": "AUTH_REQUIRED", "testLocation": "platform"})
			return
		}
		token := strings.TrimPrefix(strings.TrimSpace(h.repo.GitCredential(c.Request.Context(), user.ID)), "oauth2:")
		if token == "" {
			c.JSON(http.StatusOK, map[string]any{"status": "failed", "code": "AUTH_REQUIRED", "testLocation": "platform"})
			return
		}
		headers["Authorization"] = "Bearer " + token
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	tools, err := mcpclient.New(item.URL, headers).WithTimeout(20 * time.Second).ProbeToolsContext(ctx)
	if err != nil {
		code := "CHECK_FAILED"
		message := strings.ToLower(err.Error())
		switch {
		case strings.Contains(message, "timeout"):
			code = "TIMEOUT"
		case strings.Contains(message, "http 401"), strings.Contains(message, "http 403"), strings.Contains(message, "auth"):
			code = "AUTH_FAILED"
		case strings.Contains(message, "json-rpc"), strings.Contains(message, "initialize"):
			code = "PROTOCOL_ERROR"
		}
		c.JSON(http.StatusOK, map[string]any{"status": "failed", "code": code, "testLocation": "platform"})
		return
	}
	c.JSON(http.StatusOK, map[string]any{"status": "passed", "code": "OK", "testLocation": "platform", "testedAt": time.Now().UTC(), "tools": tools})
}

// mcpMarketInstallationUpdate edits only fields declared by a ready market
// template. Scope, ownership, market identity and template version stay fixed.
func (h *marketHandlers) mcpMarketInstallationUpdate(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	existing, ok := h.repo.MCPServer(id)
	if !ok || strings.TrimSpace(existing.MarketID) == "" {
		c.JSON(http.StatusNotFound, map[string]string{"error": "公共 MCP 配置不存在"})
		return
	}
	if existing.Scope == "project" && !h.requireProjectWriter(c, existing.ProjectID) {
		return
	}
	if existing.Scope == "personal" {
		user, authenticated := h.requestUser(c.Request, true)
		if !authenticated || user.ID != existing.OwnerUserID {
			c.JSON(http.StatusForbidden, map[string]string{"error": "不能修改其他用户的个人 MCP"})
			return
		}
	}
	var input struct {
		Name             string `json:"name"`
		TimeoutSec       int    `json:"timeoutSec"`
		ExpectedRevision int64  `json:"expectedRevision"`
		TemplateVersion  string `json:"templateVersion"`
	}
	if !decode(c, &input) {
		return
	}
	item, ok := marketItem(existing.MarketID)
	if !ok || (item.Availability != "" && item.Availability != "ready") {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "该 MCP 尚未完成平台适配"})
		return
	}
	if input.TemplateVersion != "" && input.TemplateVersion != existing.TemplateVersion {
		c.JSON(http.StatusConflict, map[string]string{"error": "MCP 模板版本已变化，请刷新目录后重试"})
		return
	}
	updated := existing
	updated.Name = strings.TrimSpace(input.Name)
	if updated.Name == "" {
		updated.Name = existing.Name
	}
	updated.TimeoutSec = input.TimeoutSec
	saved, err := h.repo.UpdateMCPServerRevision(id, input.ExpectedRevision, updated)
	if errors.Is(err, repo.ErrMCPConfigurationConflict) {
		c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	if err != nil {
		respond(c, err)
		return
	}
	c.JSON(http.StatusOK, domain.PublicMCPServer(saved))
}

// 路由: g.POST("/api/mcp-market/install", h.mcpMarketInstall)
func (h *marketHandlers) mcpMarketInstall(c *gin.Context) {
	var input struct {
		MarketID        string `json:"marketId"`
		Scope           string `json:"scope"`
		ProjectID       string `json:"projectId"`
		Name            string `json:"name"`
		TemplateVersion string `json:"templateVersion"`
	}
	if !decode(c, &input) {
		return
	}
	scope := strings.TrimSpace(input.Scope)
	ownerUserID := int64(0)
	if scope == "personal" {
		user, authenticated := h.requestUser(c.Request, true)
		if !authenticated || user.ID <= 0 {
			c.JSON(http.StatusUnauthorized, map[string]string{"error": "添加个人 MCP 前请先登录"})
			return
		}
		ownerUserID = user.ID
	} else if scope != "project" {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "请选择项目级或个人级"})
		return
	}
	item, ok := marketItem(input.MarketID)
	if !ok {
		c.JSON(http.StatusNotFound, map[string]string{"error": "MCP 市场条目不存在"})
		return
	}
	if input.TemplateVersion != "" && input.TemplateVersion != item.TemplateVersion {
		c.JSON(http.StatusConflict, map[string]string{"error": "MCP 模板版本已变化，请刷新目录后重试"})
		return
	}
	server, created, err := installMarketMCPForOwner(h.repo, input.MarketID, scope, input.ProjectID, ownerUserID)
	if err != nil {
		if strings.Contains(err.Error(), "MCP 市场条目不存在") {
			c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		if strings.Contains(err.Error(), "尚未完成平台适配") || strings.Contains(err.Error(), "个人级 MCP") || strings.Contains(err.Error(), "请选择项目级") {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if strings.Contains(err.Error(), "项目级 MCP") {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if strings.Contains(err.Error(), "项目不存在") {
			c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		respond(c, err)
		return
	}
	if !created {
		c.JSON(http.StatusConflict, map[string]any{"error": "当前范围已经添加过这个 MCP", "existingId": server.ID})
		return
	}
	h.configureSync(h.repo.SyncConfig())
	c.JSON(http.StatusCreated, domain.PublicMCPServer(server))
}

// 路由: g.POST("/api/mcp-servers/custom", h.mcpCustomInstall)
func (h *marketHandlers) mcpCustomInstall(c *gin.Context) {
	var input struct {
		Server    domain.MCPServerDefinition `json:"server"`
		Scope     string                     `json:"scope"`
		ProjectID string                     `json:"projectId"`
	}
	if !decode(c, &input) {
		return
	}
	server := input.Server
	baseID := strings.TrimSpace(server.ID)
	server.MarketID = ""
	server.Scope = strings.TrimSpace(input.Scope)
	server.ProjectID = ""
	server.OwnerUserID = 0
	ownerKey := ""
	if server.Scope == "project" {
		server.ProjectID = strings.TrimSpace(input.ProjectID)
		if server.ProjectID == "" {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "项目级 MCP 需要当前项目"})
			return
		}
		if _, exists := h.repo.Project(server.ProjectID); !exists {
			c.JSON(http.StatusNotFound, map[string]string{"error": "项目不存在"})
			return
		}
		ownerKey = server.ProjectID
	} else if server.Scope == "personal" {
		user, authenticated := h.requestUser(c.Request, true)
		if !authenticated || user.ID <= 0 {
			c.JSON(http.StatusUnauthorized, map[string]string{"error": "添加个人 MCP 前请先登录"})
			return
		}
		server.OwnerUserID = user.ID
		ownerKey = fmt.Sprintf("%d", user.ID)
	} else {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "请选择项目级或个人级"})
		return
	}
	if baseID == "" {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "自定义 MCP 需要标识"})
		return
	}
	server.ID = mcpInstallID("custom-"+baseID, server.Scope, ownerKey)
	if _, exists := h.repo.MCPServer(server.ID); exists {
		c.JSON(http.StatusConflict, map[string]string{"error": "当前范围已经存在相同标识的自定义 MCP"})
		return
	}
	if err := h.repo.PutMCPServer(server); err != nil {
		respond(c, err)
		return
	}
	h.configureSync(h.repo.SyncConfig())
	c.JSON(http.StatusCreated, domain.PublicMCPServer(server))
}

func visibleMCPServersForUser(items []domain.MCPServerDefinition, userID int64) []domain.MCPServerDefinition {
	out := make([]domain.MCPServerDefinition, 0, len(items))
	for _, item := range items {
		if item.Scope != "personal" || (userID > 0 && item.OwnerUserID == userID) {
			out = append(out, item)
		}
	}
	return out
}

func withMarketIcons(items []domain.MCPServerDefinition) []domain.MCPServerDefinition {
	out := append([]domain.MCPServerDefinition(nil), items...)
	for i := range out {
		item, ok := marketItem(out[i].MarketID)
		if ok && strings.TrimSpace(item.IconURL) != "" {
			out[i].IconURL = item.IconURL
		}
	}
	return out
}
