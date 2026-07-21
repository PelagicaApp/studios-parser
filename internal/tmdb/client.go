package tmdb

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/time/rate"

	"pelagica-studios/internal/urls"
)

// TMDB does not document a rate limit; ~40 requests per 10 seconds is the
// commonly reported figure, so requests are throttled to that rate and any
// 429 response is retried after the server-provided Retry-After delay.
const requestsPerWindow = 40
const window = 10 * time.Second
const defaultRetryAfter = window

type Client struct {
	httpClient *http.Client
	limiter    *rate.Limiter
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		limiter:    rate.NewLimiter(rate.Every(window/requestsPerWindow), requestsPerWindow),
	}
}

func (c *Client) Get(ctx context.Context, path string, queryParams map[string]string) ([]byte, error) {
	requestURL := urls.BuildURL(path, queryParams)
	headers, err := urls.BuildHeaders(urls.DefaultHeaderOptions())
	if err != nil {
		return nil, err
	}

	for {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}

		body, statusCode, retryAfter, err := c.doRequest(ctx, requestURL, headers)
		if err != nil {
			return nil, err
		}

		if statusCode == http.StatusTooManyRequests {
			select {
			case <-time.After(retryAfter):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		if statusCode != http.StatusOK {
			return nil, &StatusError{URL: requestURL, StatusCode: statusCode, Body: body}
		}

		return body, nil
	}
}

func (c *Client) doRequest(ctx context.Context, requestURL string, headers map[string]string) (body []byte, statusCode int, retryAfter time.Duration, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, 0, 0, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, 0, err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, 0, err
	}

	return body, resp.StatusCode, parseRetryAfter(resp.Header.Get("Retry-After")), nil
}

func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return defaultRetryAfter
	}
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return defaultRetryAfter
}

type StatusError struct {
	URL        string
	StatusCode int
	Body       []byte
}

func (e *StatusError) Error() string {
	return "tmdb request to " + e.URL + " failed with status " + strconv.Itoa(e.StatusCode) + ": " + string(e.Body)
}
