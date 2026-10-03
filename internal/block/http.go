package block

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type httpConfig struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// maxResponse bounds how much of a response body is read.
const maxResponse = 10 << 20

var httpClient = &http.Client{}

func runHTTP(ctx context.Context, raw json.RawMessage, env Env) (map[string]any, error) {
	var cfg httpConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	method := strings.ToUpper(cfg.Method)
	if method == "" {
		method = http.MethodGet
	}

	var body io.Reader
	if cfg.Body != "" {
		body = strings.NewReader(cfg.Body)
	}
	req, err := http.NewRequestWithContext(ctx, method, cfg.URL, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	if cfg.Body != "" && req.Header.Get("Content-Type") == "" && json.Valid([]byte(cfg.Body)) {
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(io.LimitReader(res.Body, maxResponse))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	env.Logf("%s %s → %s (%s)", method, cfg.URL, res.Status, time.Since(start).Round(time.Millisecond))

	headers := make(map[string]string, len(res.Header))
	for k := range res.Header {
		headers[k] = res.Header.Get(k)
	}
	outputs := map[string]any{
		"status":  res.StatusCode,
		"headers": headers,
		"body":    string(data),
	}
	var parsed any
	if isJSON(res.Header.Get("Content-Type")) && json.Unmarshal(data, &parsed) == nil {
		outputs["json"] = parsed
	}

	if len(data) > 0 {
		_, _ = env.Stdout.Write(append(bytes.TrimRight(data, "\n"), '\n'))
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return outputs, fmt.Errorf("request failed with %s", res.Status)
	}
	return outputs, nil
}

func isJSON(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.HasPrefix(ct, "application/json") || strings.Contains(ct, "+json")
}
