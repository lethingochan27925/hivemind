// client.go: JSON-RPC transport toi CockroachDB Managed MCP Server.
package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const protocolVersion = "2024-11-05"

type Config struct {
	Endpoint  string
	APIKey    string
	ClusterID string
	Database  string
	Timeout   time.Duration
}

type Client struct {
	cfg       Config
	http      *http.Client
	sessionID string
}

func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("mcp error %d: %s", e.Code, e.Message)
}

func (c *Client) post(payload rpcRequest) (*rpcResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.sessionID = sid
	}

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(raw))
	}

	raw, err := readSSEOrJSON(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(raw, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, rpcResp.Error
	}
	return &rpcResp, nil
}

func readSSEOrJSON(r io.Reader) ([]byte, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var raw []byte
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			raw = []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			break
		}
		if strings.HasPrefix(line, "{") {
			raw = []byte(line)
			break
		}
	}
	if raw == nil {
		return nil, fmt.Errorf("no data found in response")
	}
	return raw, nil
}

func (c *Client) Initialize() error {
	c.sessionID = ""
	_, err := c.post(rpcRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    "hivemind-agent",
				"version": "1.0",
			},
		},
		ID: 1,
	})
	return err
}

type toolInfo struct {
	Name string `json:"name"`
}

func (c *Client) ListTools() ([]string, error) {
	if err := c.Initialize(); err != nil {
		return nil, err
	}
	resp, err := c.post(rpcRequest{
		JSONRPC: "2.0",
		Method:  "tools/list",
		Params:  map[string]interface{}{},
		ID:      2,
	})
	if err != nil {
		return nil, err
	}

	var result struct {
		Tools []toolInfo `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling tools list: %w", err)
	}

	names := make([]string, len(result.Tools))
	for i, t := range result.Tools {
		names[i] = t.Name
	}
	return names, nil
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// rowsWrapper khop voi cau truc thuc te MCP tra ve: {"rows": [...]}.
type rowsWrapper struct {
	Rows json.RawMessage `json:"rows"`
}

// extractRows lay ra mang du lieu that tu response cua select_query.
// MCP tra ve dang [{"rows": [...]}] - 1 phan tu wrapper chua key "rows".
func extractRows(raw string) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(raw)

	var wrappers []rowsWrapper
	if err := json.Unmarshal([]byte(trimmed), &wrappers); err == nil && len(wrappers) > 0 {
		return wrappers[0].Rows, nil
	}

	var single rowsWrapper
	if err := json.Unmarshal([]byte(trimmed), &single); err == nil && single.Rows != nil {
		return single.Rows, nil
	}

	if strings.HasPrefix(trimmed, "[") {
		return json.RawMessage(trimmed), nil
	}
	if strings.HasPrefix(trimmed, "{") {
		return json.RawMessage("[" + trimmed + "]"), nil
	}
	return json.RawMessage("[]"), nil
}

func (c *Client) CallTool(name string, arguments map[string]interface{}) (json.RawMessage, error) {
	resp, err := c.post(rpcRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      name,
			"arguments": arguments,
		},
		ID: 2,
	})
	if err != nil {
		return nil, err
	}

	var result struct {
		Content []contentBlock `json:"content"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling tool call result: %w", err)
	}

	for _, block := range result.Content {
		if block.Type == "text" {
			return extractRows(block.Text)
		}
	}
	return json.RawMessage("[]"), nil
}

func (c *Client) Select(query string, limit int) (json.RawMessage, error) {
	if err := c.Initialize(); err != nil {
		return nil, err
	}

	safeQuery := strings.TrimSuffix(strings.TrimSpace(query), ";")
	if !strings.Contains(strings.ToLower(safeQuery), "limit") {
		safeQuery = fmt.Sprintf("%s LIMIT %d", safeQuery, limit)
	}

	return c.CallTool("select_query", map[string]interface{}{
		"cluster_id": c.cfg.ClusterID,
		"database":   c.cfg.Database,
		"query":      safeQuery,
	})
}
