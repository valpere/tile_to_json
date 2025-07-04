// internal/tile/fetcher.go - Tile fetching implementation
package tile

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/valpere/tile_to_json/internal/config"
)

// HTTPFetcher implements the Fetcher interface using HTTP requests
type HTTPFetcher struct {
	client *http.Client
	config *config.ServerConfig
}

// NewHTTPFetcher creates a new HTTP-based tile fetcher
func NewHTTPFetcher(cfg *config.Config) *HTTPFetcher {
	transport := &http.Transport{
		MaxIdleConns:        cfg.Network.MaxIdleConns,
		IdleConnTimeout:     cfg.Network.IdleConnTimeout,
		DisableKeepAlives:   cfg.Network.DisableKeepAlive,
		TLSHandshakeTimeout: 10 * time.Second,
		MaxConnsPerHost:     cfg.Batch.Concurrency,
	}

	// Configure proxy if specified
	if cfg.Network.ProxyURL != "" {
		if proxyURL, err := url.Parse(cfg.Network.ProxyURL); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	client := &http.Client{
		Timeout:   cfg.Server.Timeout,
		Transport: transport,
	}

	return &HTTPFetcher{
		client: client,
		config: &cfg.Server,
	}
}

// Fetch retrieves a single tile from the configured server
func (f *HTTPFetcher) Fetch(request *TileRequest) (*TileResponse, error) {
	start := time.Now()

	req, err := f.buildHTTPRequest(request)
	if err != nil {
		return &TileResponse{
			Request: request,
			Error:   fmt.Errorf("failed to build HTTP request: %w", err),
		}, err
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return &TileResponse{
			Request:   request,
			FetchTime: time.Since(start),
			Error:     fmt.Errorf("HTTP request failed: %w", err),
		}, err
	}
	defer resp.Body.Close()

	// Handle compressed responses
	var reader io.Reader = resp.Body
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return &TileResponse{
				Request:    request,
				StatusCode: resp.StatusCode,
				Headers:    resp.Header,
				FetchTime:  time.Since(start),
				Error:      fmt.Errorf("failed to create gzip reader: %w", err),
			}, err
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return &TileResponse{
			Request:    request,
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			FetchTime:  time.Since(start),
			Error:      fmt.Errorf("failed to read response body: %w", err),
		}, err
	}

	response := &TileResponse{
		Request:    request,
		Data:       data,
		Headers:    resp.Header,
		StatusCode: resp.StatusCode,
		Size:       len(data),
		FetchTime:  time.Since(start),
	}

	if resp.StatusCode != http.StatusOK {
		response.Error = fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		return response, response.Error
	}

	return response, nil
}

// FetchWithRetry implements retry logic for failed tile requests
func (f *HTTPFetcher) FetchWithRetry(request *TileRequest) (*TileResponse, error) {
	var lastResponse *TileResponse
	var lastErr error

	for attempt := 0; attempt <= f.config.MaxRetries; attempt++ {
		if attempt > 0 {
			backoffDelay := time.Duration(attempt*attempt) * time.Second
			time.Sleep(backoffDelay)
		}

		response, err := f.Fetch(request)
		if err == nil {
			return response, nil
		}

		lastResponse = response
		lastErr = err

		// Determine if we should retry based on the error type
		if !f.shouldRetry(response, err) {
			break
		}
	}

	return lastResponse, fmt.Errorf("failed after %d attempts: %w", f.config.MaxRetries+1, lastErr)
}

// buildHTTPRequest constructs an HTTP request from a tile request
func (f *HTTPFetcher) buildHTTPRequest(tileReq *TileRequest) (*http.Request, error) {
	req, err := http.NewRequest("GET", tileReq.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set default headers
	req.Header.Set("Accept", "application/x-protobuf")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("User-Agent", "TileToJson/1.0")

	// Add authentication if configured
	if f.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+f.config.APIKey)
	}

	// Add server-level headers from configuration
	for key, value := range f.config.Headers {
		req.Header.Set(key, value)
	}

	// Add request-specific headers
	for key, value := range tileReq.Headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

// shouldRetry determines whether a failed request should be retried
func (f *HTTPFetcher) shouldRetry(response *TileResponse, err error) bool {
	// Always retry on network errors
	if response == nil {
		return true
	}

	// Don't retry on client errors (4xx)
	if response.StatusCode >= 400 && response.StatusCode < 500 {
		return false
	}

	// Retry on server errors (5xx) and timeout errors
	if response.StatusCode >= 500 || response.StatusCode == 0 {
		return true
	}

	return false
}

// FetchBatch fetches multiple tiles concurrently
func (f *HTTPFetcher) FetchBatch(requests []*TileRequest, concurrency int) ([]*TileResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), f.config.Timeout*time.Duration(len(requests)))
	defer cancel()

	requestChan := make(chan *TileRequest, len(requests))
	responseChan := make(chan *TileResponse, len(requests))

	// Send all requests to the channel
	for _, req := range requests {
		requestChan <- req
	}
	close(requestChan)

	// Start worker goroutines
	for i := 0; i < concurrency; i++ {
		go func() {
			for req := range requestChan {
				select {
				case <-ctx.Done():
					responseChan <- &TileResponse{
						Request: req,
						Error:   ctx.Err(),
					}
					return
				default:
					response, _ := f.FetchWithRetry(req)
					responseChan <- response
				}
			}
		}()
	}

	// Collect responses
	responses := make([]*TileResponse, 0, len(requests))
	for i := 0; i < len(requests); i++ {
		response := <-responseChan
		responses = append(responses, response)
	}

	return responses, nil
}
