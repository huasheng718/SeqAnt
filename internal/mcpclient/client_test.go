package mcpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListToolsPreservesSchema(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Method string `json:"method"`
		}
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		if req.Method == "tools/list" {
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"stories_get","description":"get","inputSchema":{"type":"object","required":["id"]},"annotations":{"readOnlyHint":true}}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
	t.Cleanup(srv.Close)
	tools, err := New(srv.URL, nil).ListTools()
	if err != nil || len(tools) != 1 {
		t.Fatalf("list: %v %+v", err, tools)
	}
	if !bytes.Contains(tools[0].InputSchema, []byte(`"required"`)) || !bytes.Contains(tools[0].Annotations, []byte("readOnlyHint")) {
		t.Fatalf("schema not preserved: %+v", tools[0])
	}
}

func TestProbeToolsIsStrictAboutInitialization(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		if req.Method == "initialize" {
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"bad auth"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"tools":[]}}`))
	}))
	t.Cleanup(srv.Close)
	if _, err := New(srv.URL, nil).ProbeToolsContext(context.Background()); err == nil {
		t.Fatal("strict probe accepted initialization error")
	}
}

func TestProbeToolsCarriesSessionAndPaginates(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req struct {
			Method string         `json:"method"`
			ID     int64          `json:"id"`
			Params map[string]any `json:"params"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		if req.Method == "initialize" {
			w.Header().Set("Mcp-Session-Id", "session-1")
			_, _ = w.Write([]byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{}}`, req.ID)))
			return
		}
		if req.Method == "notifications/initialized" {
			if r.Header.Get("Mcp-Session-Id") != "session-1" {
				t.Errorf("missing session header")
			}
			_, _ = w.Write([]byte(`{}`))
			return
		}
		cursor, _ := req.Params["cursor"].(string)
		if cursor == "" {
			_, _ = w.Write([]byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"tools":[{"name":"one"}],"nextCursor":"next"}}`, req.ID)))
			return
		}
		if r.Header.Get("Mcp-Session-Id") != "session-1" {
			t.Errorf("missing session header on page")
		}
		_, _ = w.Write([]byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"result":{"tools":[{"name":"two"}]}}`, req.ID)))
	}))
	t.Cleanup(srv.Close)
	tools, err := New(srv.URL, nil).ProbeToolsContext(context.Background())
	if err != nil || len(tools) != 2 || calls < 4 {
		t.Fatalf("tools=%+v err=%v calls=%d", tools, err, calls)
	}
}

func TestProbeToolsRejectsRedirects(t *testing.T) {
	var target *httptest.Server
	target = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", target.URL+"/other")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	t.Cleanup(target.Close)
	if _, err := New(target.URL, nil).ProbeToolsContext(context.Background()); err == nil {
		t.Fatal("strict probe followed redirect")
	}
}

func TestProbeToolsBoundsResponseSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"padding":"`+strings.Repeat("x", 1<<20)+`"}}`)
	}))
	t.Cleanup(srv.Close)
	if _, err := New(srv.URL, nil).ProbeToolsContext(context.Background()); err == nil {
		t.Fatal("strict probe accepted oversized response")
	}
}
