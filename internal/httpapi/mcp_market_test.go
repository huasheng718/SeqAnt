package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agentflow/internal/domain"
	"agentflow/internal/engine"
	"agentflow/internal/repo"
	"agentflow/internal/runtime"
	"agentflow/internal/tracker"
	webassets "agentflow/web"

	"github.com/gin-gonic/gin"
)

func assertProjectHasDefaultTrackerMCPs(t *testing.T, st *repo.Store, projectID string) {
	t.Helper()
	for _, server := range st.MCPServers() {
		if server.ProjectID == projectID && (server.MarketID == "61" || server.MarketID == "70") {
			t.Fatalf("public startup installed an enterprise MCP: %+v", server)
		}
	}
}
func installTestTrackerMCPs(t *testing.T, st *repo.Store, projectID string) {
	t.Helper()
	for _, item := range []struct{ id, name string }{{"61", "gongfeng"}, {"70", "tapd"}} {
		if err := st.PutMCPServer(domain.MCPServerDefinition{ID: mcpInstallID(item.id, "project", projectID), Name: item.name, MarketID: item.id, Scope: "project", ProjectID: projectID, Transport: "http", URL: "https://mcp.example.com/" + item.name}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMCPInstallIDSeparatesScopeAndOwner(t *testing.T) {
	project := mcpInstallID("wiki", "project", "platform")
	personal := mcpInstallID("wiki", "personal", "2")
	if project == personal || project != "mcp-wiki-project-platform" || personal != "mcp-wiki-personal-2" {
		t.Fatalf("unexpected install ids: project=%q personal=%q", project, personal)
	}
}

func TestMCPInstallIDBoundsLongNamesAndOwners(t *testing.T) {
	name, owner := strings.Repeat("custom-name-", 10), strings.Repeat("project-", 10)
	one := mcpInstallID(name, "project", owner)
	if len(one) != 64 || one != mcpInstallID(name, "project", owner) {
		t.Fatalf("unstable or oversized ID: %s", one)
	}
	if one == mcpInstallID(name, "project", owner+"other") || one == mcpInstallID(name, "personal", owner) {
		t.Fatal("scope or owner lost in bounded ID")
	}
}

func TestMCPMarketCatalogContainsOnlyPublicServices(t *testing.T) {
	if len(mcpMarketItems) != 5 {
		t.Fatalf("catalog: %+v", mcpMarketItems)
	}
	item, ok := marketItem("github")
	if !ok || item.URL != "https://api.githubcopilot.com/mcp/" || item.AuthType != "github-token" {
		t.Fatalf("GitHub: %+v", item)
	}
	if item, ok := marketItem("filesystem"); !ok || item.Availability != "candidate" {
		t.Fatal("candidate catalog metadata missing")
	}
}

func TestMCPMarketSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &marketHandlers{}
	req := httptest.NewRequest(http.MethodGet, "/api/mcp-market?q=github", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	h.mcpMarket(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Source string          `json:"source"`
		Total  int             `json:"total"`
		Items  []mcpMarketItem `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Source != "loopforge-public" || payload.Total != len(mcpMarketItems) {
		t.Fatalf("catalog meta = %+v", payload)
	}
	if len(payload.Items) == 0 {
		t.Fatal("expected search hits for github")
	}
	found := false
	for _, item := range payload.Items {
		if item.ID == "github" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("search github missed id github: %+v", payload.Items)
	}

	all := httptest.NewRecorder()
	allCtx, _ := gin.CreateTestContext(all)
	allCtx.Request = httptest.NewRequest(http.MethodGet, "/api/mcp-market", nil)
	h.mcpMarket(allCtx)
	var allPayload struct {
		Items []mcpMarketItem `json:"items"`
	}
	if err := json.Unmarshal(all.Body.Bytes(), &allPayload); err != nil {
		t.Fatal(err)
	}
	if len(allPayload.Items) != len(mcpMarketItems) {
		t.Fatalf("unfiltered items=%d catalog=%d", len(allPayload.Items), len(mcpMarketItems))
	}
}

func TestMCPMarketInstallAliasStoresCanonicalID(t *testing.T) {
	st, err := repo.OpenTest()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.PutProject(domain.Project{ID: "platform", Name: "Platform"}); err != nil {
		t.Fatal(err)
	}
	h := &marketHandlers{deps: &deps{repo: st}}
	newInstallRecorder := func(body string) (*gin.Context, *httptest.ResponseRecorder) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/mcp-market/install", bytes.NewBufferString(body))
		return c, rec
	}
	c, rec := newInstallRecorder(`{"marketId":"github","scope":"project","projectId":"platform"}`)
	h.mcpMarketInstall(c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var saved domain.MCPServerDefinition
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.MarketID != "github" || saved.Name != "GitHub" {
		t.Fatalf("saved = %+v", saved)
	}
	conflictCtx, conflictRec := newInstallRecorder(`{"marketId":"github","scope":"project","projectId":"platform"}`)
	h.mcpMarketInstall(conflictCtx)
	if conflictRec.Code != http.StatusConflict {
		t.Fatalf("expected conflict for alias/canonical, status=%d body=%s", conflictRec.Code, conflictRec.Body.String())
	}
}

func TestMCPMarketCheckPerformsStrictReadOnlyProbe(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			ID     int64  `json:"id"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05"}}`))
		case "tools/list":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"read_docs"}]}}`))
		default:
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":0,"result":{}}`))
		}
	}))
	t.Cleanup(upstream.Close)
	saved := mcpMarketItems
	t.Cleanup(func() { mcpMarketItems = saved })
	mcpMarketItems = []mcpMarketItem{{ID: "probe", Name: "Probe", URL: upstream.URL, Transport: "http", Availability: "ready", TemplateVersion: "1"}}
	h := &marketHandlers{deps: &deps{repo: nil}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/mcp-market/check", bytes.NewBufferString(`{"marketId":"probe","scope":"project","templateVersion":"1"}`))
	h.mcpMarketCheck(c)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"passed"`) || !strings.Contains(rec.Body.String(), `read_docs`) {
		t.Fatalf("probe status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMCPMarketInstallationUpdateUsesRevision(t *testing.T) {
	st, err := repo.OpenTest()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	if err := st.PutProject(domain.Project{ID: "p1", Name: "P1"}); err != nil {
		t.Fatal(err)
	}
	server := domain.MCPServerDefinition{ID: "mcp-github-project-p1", Name: "GitHub", Scope: "project", ProjectID: "p1", MarketID: "github", TemplateVersion: "1", Transport: "http", URL: "https://example.test/mcp"}
	if err := st.PutMCPServer(server); err != nil {
		t.Fatal(err)
	}
	h := &marketHandlers{deps: &deps{repo: st}}
	g := gin.New()
	g.PUT("/api/mcp-market/installations/:id", h.mcpMarketInstallationUpdate)
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/mcp-market/installations/"+server.ID, bytes.NewBufferString(`{"name":"GitHub docs","timeoutSec":45,"expectedRevision":0,"templateVersion":"1"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var saved domain.MCPServerDefinition
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Name != "GitHub docs" || saved.ConfigurationRevision != 1 || saved.TimeoutSec != 45 {
		t.Fatalf("saved=%+v", saved)
	}
	stale := httptest.NewRecorder()
	g.ServeHTTP(stale, httptest.NewRequest(http.MethodPut, "/api/mcp-market/installations/"+server.ID, bytes.NewBufferString(`{"name":"stale","timeoutSec":30,"expectedRevision":0,"templateVersion":"1"}`)))
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale status=%d body=%s", stale.Code, stale.Body.String())
	}
}

func TestEnsureDefaultProjectMCPsIsIdempotent(t *testing.T) {
	st, err := repo.OpenTest()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.PutProject(domain.Project{ID: "onboard", Name: "Onboard"}); err != nil {
		t.Fatal(err)
	}
	if err := ensureDefaultProjectMCPs(st, "onboard"); err != nil {
		t.Fatal(err)
	}
	assertProjectHasDefaultTrackerMCPs(t, st, "onboard")
	before := len(st.MCPServers())
	if err := ensureDefaultProjectMCPs(st, "onboard"); err != nil {
		t.Fatal(err)
	}
	if got := len(st.MCPServers()); got != before {
		t.Fatalf("second ensure created duplicates: %d -> %d", before, got)
	}
}

func TestNewBackfillsDefaultMCPsOnExistingProjects(t *testing.T) {
	st, err := repo.OpenTest()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.PutProject(domain.Project{ID: "legacy", Name: "Legacy"}); err != nil {
		t.Fatal(err)
	}
	New(st, engine.New(st, runtime.NewManager(t.TempDir())), tracker.NewService(), "", "")
	assertProjectHasDefaultTrackerMCPs(t, st, "legacy")
	assertProjectHasDefaultTrackerMCPs(t, st, "platform")
}

func TestMarketMCPRejectsPersonalScope(t *testing.T) {
	st, err := repo.OpenTest()
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{deps: &deps{repo: st}}
	s.market = &marketHandlers{deps: s.deps}
	g := gin.New()
	g.POST("/api/mcp-market/install", s.market.mcpMarketInstall)
	req := httptest.NewRequest(http.MethodPost, "/api/mcp-market/install", bytes.NewBufferString(`{"marketId":"github","scope":"personal"}`))
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestVisibleMCPServersForUserHidesOtherPersonalItems(t *testing.T) {
	items := []domain.MCPServerDefinition{
		{ID: "project", Scope: "project", ProjectID: "p1"},
		{ID: "mine", Scope: "personal", OwnerUserID: 2},
		{ID: "other", Scope: "personal", OwnerUserID: 3},
	}
	visible := visibleMCPServersForUser(items, 2)
	if len(visible) != 2 || visible[0].ID != "project" || visible[1].ID != "mine" {
		t.Fatalf("visible items = %#v", visible)
	}
}

func TestUpdateCustomMCPKeepsScopeAndClearsIcon(t *testing.T) {
	st, err := repo.OpenTest()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	original := domain.MCPServerDefinition{ID: "custom", Name: "Old", Scope: "project", ProjectID: "gslb", IconURL: "https://broken.example/icon.svg", Transport: "http", URL: "https://old.example/mcp"}
	if err := st.PutMCPServer(original); err != nil {
		t.Fatal(err)
	}
	s := &Server{deps: &deps{repo: st}}
	s.mcp = &mcpHandlers{deps: s.deps}
	g := gin.New()
	g.PUT("/api/mcp-servers", s.mcp.mcpServers)
	req := httptest.NewRequest(http.MethodPut, "/api/mcp-servers", bytes.NewBufferString(`{"id":"custom","name":"New","scope":"personal","ownerUserId":99,"projectId":"other","iconUrl":"https://bad.example/icon.svg","transport":"http","url":"https://new.example/mcp"}`))
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var saved domain.MCPServerDefinition
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Name != "New" || saved.Scope != "project" || saved.ProjectID != "gslb" || saved.OwnerUserID != 0 || saved.IconURL != "" {
		t.Fatalf("saved MCP = %+v", saved)
	}
}

func TestUpdateMarketMCPIsRejected(t *testing.T) {
	st, err := repo.OpenTest()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.PutMCPServer(domain.MCPServerDefinition{ID: "market", Name: "Market", MarketID: "gongfeng", Scope: "project", ProjectID: "platform", Transport: "http", URL: "https://example.com/mcp"}); err != nil {
		t.Fatal(err)
	}
	s := &Server{deps: &deps{repo: st}}
	s.mcp = &mcpHandlers{deps: s.deps}
	g := gin.New()
	g.PUT("/api/mcp-servers", s.mcp.mcpServers)
	req := httptest.NewRequest(http.MethodPut, "/api/mcp-servers", bytes.NewBufferString(`{"id":"market","name":"Changed","transport":"http","url":"https://changed.example/mcp"}`))
	rec := httptest.NewRecorder()
	g.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMCPMarketIconsAreLocalAndEmbedded(t *testing.T) {
	static, err := fs.Sub(webassets.Files, ".")
	if err != nil {
		t.Fatal(err)
	}
	var withIcon int
	for _, item := range mcpMarketItems {
		if strings.TrimSpace(item.IconURL) == "" {
			continue
		}
		withIcon++
		if !strings.HasPrefix(item.IconURL, "/mcp-icons/") {
			t.Fatalf("%s iconUrl = %q, want local /mcp-icons/", item.ID, item.IconURL)
		}
		name := strings.TrimPrefix(item.IconURL, "/mcp-icons/")
		if _, err := fs.ReadFile(static, "mcp-icons/"+name); err != nil {
			t.Fatalf("embedded icon %s: %v", name, err)
		}
	}
	if withIcon < 1 {
		t.Fatalf("expected local icons, got %d", withIcon)
	}
}
