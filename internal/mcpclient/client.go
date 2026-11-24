package mcpclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client is a minimal Streamable HTTP MCP client that sends JSON-RPC 2.0
// tool-call requests to an MCP gateway and parses the response.
type Client struct {
	url       string
	hdrs      map[string]string
	http      *http.Client
	mu        sync.Mutex
	nextID    int64
	init      bool
	sessionID string
}

func New(url string, headers map[string]string) *Client {
	return &Client{
		url:  strings.TrimRight(url, "/"),
		hdrs: headers,
		http: &http.Client{Timeout: 60 * time.Second},
	}
}

// SetHeaders updates the HTTP headers (e.g., auth tokens) at runtime.
func (c *Client) SetHeaders(headers map[string]string) {
	c.hdrs = headers
}

// Header returns one request header, or empty if the client is nil.
func (c *Client) Header(key string) string {
	if c == nil {
		return ""
	}
	for k, v := range c.hdrs {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

// Clone returns a client that shares the same URL and headers with an optional
// request timeout. The original client is not mutated.
func (c *Client) Clone(timeout time.Duration) *Client {
	if c == nil {
		return nil
	}
	return New(c.url, cloneHeaders(c.hdrs)).WithTimeout(timeout)
}

// WithTimeout sets the HTTP timeout on this client and returns it.
func (c *Client) WithTimeout(timeout time.Duration) *Client {
	if c != nil && timeout > 0 {
		c.http.Timeout = timeout
	}
	return c
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError,omitempty"`
}

// CallTool invokes a named MCP tool with the given arguments and returns the
// text content of the first result block.
func (c *Client) CallTool(name string, args map[string]any) (string, error) {
	return c.CallToolContext(context.Background(), name, args)
}

// CallToolContext is CallTool with a caller-supplied deadline/cancellation.
func (c *Client) CallToolContext(ctx context.Context, name string, args map[string]any) (string, error) {
	if err := c.ensureInit(); err != nil {
		return "", fmt.Errorf("mcp init: %w", err)
	}
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      name,
			"arguments": args,
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	if ctx == nil {
		ctx = context.Background()
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range c.hdrs {
		httpReq.Header.Set(k, v)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("mcp request %s: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		return "", fmt.Errorf("gateway HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	raw, err := c.readResponse(resp)
	if err != nil {
		return "", fmt.Errorf("mcp response %s: %w", name, err)
	}

	var gatewayErr struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &gatewayErr) == nil && gatewayErr.Error != "" {
		var check map[string]json.RawMessage
		if json.Unmarshal(raw, &check) == nil {
			if _, hasResult := check["result"]; !hasResult {
				if _, hasID := check["id"]; !hasID {
					return "", fmt.Errorf("mcp gateway error: %s", gatewayErr.Error)
				}
			}
		}
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(raw, &rpcResp); err != nil {
		return "", fmt.Errorf("mcp decode %s: %w (body: %s)", name, err, string(raw))
	}
	if rpcResp.Error != nil {
		return "", fmt.Errorf("mcp tool %s error [%d]: %s", name, rpcResp.Error.Code, rpcResp.Error.Message)
	}

	var result toolCallResult
	if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
		return strings.TrimSpace(string(rpcResp.Result)), nil
	}
	if result.IsError && len(result.Content) > 0 {
		return "", fmt.Errorf("mcp tool %s: %s", name, result.Content[0].Text)
	}
	var textBlocks []string
	for _, block := range result.Content {
		if block.Type == "text" && block.Text != "" {
			textBlocks = append(textBlocks, block.Text)
		}
	}
	return strings.Join(textBlocks, "\n"), nil
}

type Tool struct {
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	InputSchema  json.RawMessage `json:"inputSchema,omitempty"`
	OutputSchema json.RawMessage `json:"outputSchema,omitempty"`
	Annotations  json.RawMessage `json:"annotations,omitempty"`
}

// ListTools returns the remote MCP tool catalog after initialize.
func (c *Client) ListTools() ([]Tool, error) {
	return c.ListToolsContext(context.Background())
}

// ListToolsContext is ListTools with a caller-supplied deadline/cancellation.
func (c *Client) ListToolsContext(ctx context.Context) ([]Tool, error) {
	if err := c.ensureInit(); err != nil {
		return nil, fmt.Errorf("mcp init: %w", err)
	}
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()
	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/list",
		Params:  map[string]any{},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range c.hdrs {
		httpReq.Header.Set(k, v)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mcp tools/list: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		return nil, fmt.Errorf("gateway HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	raw, err := c.readResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("mcp tools/list: %w", err)
	}
	var rpcResp rpcResponse
	if err := json.Unmarshal(raw, &rpcResp); err != nil {
		return nil, fmt.Errorf("mcp decode tools/list: %w (body: %s)", err, string(raw))
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("mcp tools/list error [%d]: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
		return nil, fmt.Errorf("mcp tools/list result: %w", err)
	}
	return result.Tools, nil
}

// ProbeToolsContext performs a strict, read-only MCP handshake followed by
// tools/list. Unlike the legacy runtime path, initialization failures and
// malformed responses are errors rather than a best-effort continuation.
func (c *Client) ProbeToolsContext(ctx context.Context) ([]Tool, error) {
	if c == nil {
		return nil, fmt.Errorf("nil MCP client")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	c.mu.Lock()
	c.nextID++
	initID := c.nextID
	c.nextID++
	toolsID := c.nextID
	c.mu.Unlock()
	initRaw, err := c.strictRPC(ctx, rpcRequest{JSONRPC: "2.0", ID: initID, Method: "initialize", Params: map[string]any{
		"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "agentflow-check", "version": "1"},
	}})
	if err != nil {
		return nil, fmt.Errorf("mcp initialize: %w", err)
	}
	var initResp rpcResponse
	if err := json.Unmarshal(initRaw, &initResp); err != nil || initResp.Result == nil {
		return nil, fmt.Errorf("mcp initialize returned an invalid result")
	}
	if err := c.strictNotification(ctx, rpcRequest{JSONRPC: "2.0", Method: "notifications/initialized", Params: map[string]any{}}); err != nil {
		return nil, fmt.Errorf("mcp initialized notification: %w", err)
	}
	tools := make([]Tool, 0)
	cursor := ""
	for page := 0; page < 20; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		toolsRaw, err := c.strictRPC(ctx, rpcRequest{JSONRPC: "2.0", ID: toolsID + int64(page), Method: "tools/list", Params: params})
		if err != nil {
			return nil, err
		}
		var toolsResp rpcResponse
		if err := json.Unmarshal(toolsRaw, &toolsResp); err != nil {
			return nil, fmt.Errorf("mcp tools/list response: %w", err)
		}
		var result struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(toolsResp.Result, &result); err != nil {
			return nil, fmt.Errorf("mcp tools/list result: %w", err)
		}
		tools = append(tools, result.Tools...)
		if len(tools) > 1000 {
			return nil, fmt.Errorf("mcp tools/list response limit exceeded")
		}
		if result.NextCursor == "" {
			return tools, nil
		}
		cursor = result.NextCursor
	}
	return nil, fmt.Errorf("mcp tools/list pagination limit exceeded")
}

func (c *Client) strictRPC(ctx context.Context, request rpcRequest) ([]byte, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for key, value := range c.hdrs {
		req.Header.Set(key, value)
	}
	c.mu.Lock()
	sessionID := c.sessionID
	c.mu.Unlock()
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	client := *c.http
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gateway HTTP %d", resp.StatusCode)
	}
	if sessionID := resp.Header.Get("Mcp-Session-Id"); sessionID != "" {
		c.mu.Lock()
		c.sessionID = sessionID
		c.mu.Unlock()
	}
	raw, err := c.readResponseLimited(resp, 1<<20)
	if err != nil {
		return nil, err
	}
	var decoded rpcResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("invalid JSON-RPC response: %w", err)
	}
	if decoded.ID != request.ID {
		return nil, fmt.Errorf("JSON-RPC response id mismatch")
	}
	if decoded.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error [%d]: %s", decoded.Error.Code, decoded.Error.Message)
	}
	if decoded.Result == nil {
		return nil, fmt.Errorf("JSON-RPC response has no result")
	}
	return raw, nil
}

func (c *Client) readResponseLimited(resp *http.Response, max int64) ([]byte, error) {
	var raw []byte
	var err error
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		raw, err = c.readSSE(io.LimitReader(resp.Body, max+1))
	} else {
		raw, err = io.ReadAll(io.LimitReader(resp.Body, max+1))
	}
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > max {
		return nil, fmt.Errorf("MCP response exceeds %d bytes", max)
	}
	return raw, nil
}

func (c *Client) strictNotification(ctx context.Context, request rpcRequest) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for key, value := range c.hdrs {
		req.Header.Set(key, value)
	}
	c.mu.Lock()
	sessionID := c.sessionID
	c.mu.Unlock()
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	client := *c.http
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gateway HTTP %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) ensureInit() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.init {
		return nil
	}
	c.nextID++
	id := c.nextID

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "agentflow",
				"version": "1.0.0",
			},
		},
	}
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", c.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range c.hdrs {
		httpReq.Header.Set(k, v)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		c.init = true
		return nil
	}
	defer resp.Body.Close()
	raw, err := c.readResponse(resp)
	if err != nil {
		c.init = true
		return nil
	}
	var rpcResp rpcResponse
	if json.Unmarshal(raw, &rpcResp) == nil && rpcResp.Error != nil {
		// Non-fatal: continue even if init fails.
	}

	c.nextID++
	notifReq := rpcRequest{
		JSONRPC: "2.0",
		ID:      0,
		Method:  "notifications/initialized",
	}
	notifBody, _ := json.Marshal(notifReq)
	notifHTTP, _ := http.NewRequest("POST", c.url, bytes.NewReader(notifBody))
	notifHTTP.Header.Set("Content-Type", "application/json")
	notifHTTP.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range c.hdrs {
		notifHTTP.Header.Set(k, v)
	}
	notifResp, err := c.http.Do(notifHTTP)
	if err == nil {
		notifResp.Body.Close()
	}
	c.init = true
	return nil
}

func (c *Client) readResponse(resp *http.Response) ([]byte, error) {
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		return c.readSSE(resp.Body)
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) readSSE(body io.Reader) ([]byte, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var dataLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if len(dataLines) > 0 {
				data := strings.Join(dataLines, "\n")
				dataLines = dataLines[:0]
				var rpcResp rpcResponse
				if json.Unmarshal([]byte(data), &rpcResp) == nil && rpcResp.ID > 0 {
					if rpcResp.Result != nil {
						return json.Marshal(rpcResp)
					}
					if rpcResp.Error != nil {
						return json.Marshal(rpcResp)
					}
				}
			}
			continue
		}
		if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		}
	}
	if len(dataLines) > 0 {
		return []byte(strings.Join(dataLines, "\n")), nil
	}
	return nil, fmt.Errorf("no data in SSE stream")
}

func cloneHeaders(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
