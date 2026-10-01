package nobusapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultEndpoint = "https://cloud-api.nobus.io"
	defaultTimeout  = 2 * time.Minute
	defaultPoll     = 2 * time.Second
)

var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrInvalid      = errors.New("invalid request")
	ErrUnavailable  = errors.New("unavailable")
)

type Client struct {
	base   *url.URL
	http   *http.Client
	apiKey string
	poll   time.Duration
}

type Config struct {
	Endpoint   string
	APIKey     string
	HTTPClient *http.Client
	Poll       time.Duration
}

func NewClient(cfg Config) (*Client, error) {
	endpoint := strings.TrimRight(cfg.Endpoint, "/")
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	base, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse Nobus API URL: %w", err)
	}
	if base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("nobus API URL must be absolute")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("nobus API key is required")
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	poll := cfg.Poll
	if poll <= 0 {
		poll = defaultPoll
	}
	return &Client{base: base, http: httpClient, apiKey: cfg.APIKey, poll: poll}, nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	const maxAttempts = 4
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode Nobus request: %w", err)
		}
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := c.doOnce(ctx, method, path, query, payload, out)
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt == maxAttempts || !errors.Is(err, ErrUnavailable) || ctx.Err() != nil {
			return err
		}
		backoff := time.Duration(attempt*attempt) * 200 * time.Millisecond
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("call Nobus API: %w", ctx.Err())
		case <-timer.C:
		}
	}
	return lastErr
}

func (c *Client) doOnce(ctx context.Context, method, path string, query url.Values, payload []byte, out any) error {
	endpoint := c.base.ResolveReference(&url.URL{Path: path})
	if query != nil {
		endpoint.RawQuery = query.Encode()
	}

	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
	if err != nil {
		return fmt.Errorf("create Nobus request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call Nobus API: %w", errors.Join(ErrUnavailable, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return mapHTTPError(resp)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode Nobus response: %w", errors.Join(ErrInvalid, err))
	}
	return nil
}

func projectQuery(projectID, availabilityZone string) url.Values {
	q := url.Values{}
	q.Set("project_id", projectID)
	if availabilityZone != "" {
		q.Set("availability_zone", availabilityZone)
	}
	return q
}

type errorResponse struct {
	Message string `json:"message"`
}

func mapHTTPError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read Nobus error response: %w", errors.Join(ErrUnavailable, err))
	}
	var apiErr errorResponse
	if len(body) > 0 {
		if err := json.Unmarshal(body, &apiErr); err != nil {
			apiErr.Message = strings.TrimSpace(string(body))
		}
	}
	message := apiErr.Message
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	switch resp.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("%s: %w", message, ErrNotFound)
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%s: %w", message, ErrUnauthorized)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%s: %w", message, ErrUnavailable)
	default:
		if resp.StatusCode >= http.StatusInternalServerError {
			return fmt.Errorf("%s: %w", message, ErrUnavailable)
		}
		// Nobus sometimes reports a missing resource as HTTP 400 with this text.
		lower := strings.ToLower(message)
		if strings.Contains(lower, "not found") || strings.Contains(lower, "does not exist") {
			return fmt.Errorf("%s: %w", message, ErrNotFound)
		}
		return fmt.Errorf("%s: %w", message, ErrInvalid)
	}
}

// decodeList accepts the two list shapes Nobus returns: a JSON array, or an object with an "items" array.
func decodeList[T any](raw json.RawMessage) ([]T, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	switch raw[0] {
	case '[':
		var items []T
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, fmt.Errorf("decode list payload: %w", err)
		}
		return items, nil
	case '{':
		var wrapped struct {
			Items json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(raw, &wrapped); err != nil {
			return nil, fmt.Errorf("decode list payload: %w", err)
		}
		if len(bytes.TrimSpace(wrapped.Items)) == 0 {
			return nil, fmt.Errorf("decode list payload: %w", ErrInvalid)
		}
		if bytes.Equal(bytes.TrimSpace(wrapped.Items), []byte("null")) {
			return nil, nil
		}
		var items []T
		if err := json.Unmarshal(wrapped.Items, &items); err != nil {
			return nil, fmt.Errorf("decode list payload: %w", err)
		}
		return items, nil
	default:
		return nil, fmt.Errorf("decode list payload: %w", ErrInvalid)
	}
}

func (c *Client) waitUntil(ctx context.Context, timeout time.Duration, fn func(context.Context) (bool, error)) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()
	for {
		done, err := fn(ctx)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		timer.Reset(c.poll)
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait timed out: %w", ctx.Err())
		case <-timer.C:
		}
	}
}
