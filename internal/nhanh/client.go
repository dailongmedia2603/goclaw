package nhanh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"time"

	"golang.org/x/time/rate"
)

const (
	baseURL         = "https://pos.open.nhanh.vn"
	defaultPageSize = 20
)

// Client is a Nhanh.vn API v3.0 HTTP client.
type Client struct {
	httpClient  *http.Client
	appID       string
	businessID  string
	accessToken string
	limiter     *rate.Limiter
}

// NewClient creates a new Nhanh.vn API client with rate limiting (150 req/30s).
func NewClient(appID, businessID, accessToken string) *Client {
	return &Client{
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		appID:       appID,
		businessID:  businessID,
		accessToken: accessToken,
		limiter:     rate.NewLimiter(rate.Every(30*time.Second/150), 10),
	}
}

// ListProducts fetches products with optional filters and pagination.
func (c *Client) ListProducts(ctx context.Context, filters *ProductFilters, page *PaginatorInput) ([]Product, *Paginator, error) {
	req := buildListRequest(filters, page)
	resp, err := c.post(ctx, "/v3.0/product/list", req)
	if err != nil {
		return nil, nil, err
	}
	var products []Product
	if err := json.Unmarshal(resp.Data, &products); err != nil {
		return nil, nil, fmt.Errorf("nhanh: parse products: %w", err)
	}
	return products, resp.Paginator, nil
}

// ListOrders fetches orders with optional filters and pagination.
func (c *Client) ListOrders(ctx context.Context, filters *OrderFilters, page *PaginatorInput) ([]Order, *Paginator, error) {
	req := buildListRequest(filters, page)
	resp, err := c.post(ctx, "/v3.0/order/list", req)
	if err != nil {
		return nil, nil, err
	}
	var orders []Order
	if err := json.Unmarshal(resp.Data, &orders); err != nil {
		return nil, nil, fmt.Errorf("nhanh: parse orders: %w", err)
	}
	return orders, resp.Paginator, nil
}

// ListCustomers fetches customers with optional filters and pagination.
func (c *Client) ListCustomers(ctx context.Context, filters *CustomerFilters, page *PaginatorInput) ([]Customer, *Paginator, error) {
	req := buildListRequest(filters, page)
	resp, err := c.post(ctx, "/v3.0/customer/list", req)
	if err != nil {
		return nil, nil, err
	}
	var customers []Customer
	if err := json.Unmarshal(resp.Data, &customers); err != nil {
		return nil, nil, fmt.Errorf("nhanh: parse customers: %w", err)
	}
	return customers, resp.Paginator, nil
}

// buildListRequest creates a ListRequest, ensuring filters is never null in JSON.
// Nhanh.vn API rejects "filters":null — must be {} or a valid object.
func buildListRequest(filters any, page *PaginatorInput) ListRequest {
	if isNilFilter(filters) {
		filters = struct{}{}
	}
	return ListRequest{Filters: filters, Paginator: page}
}

// isNilFilter checks if a filter pointer is nil (handles typed nil interface values).
func isNilFilter(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Ptr && rv.IsNil()
}

// post sends a POST request to the Nhanh.vn API and returns the parsed response.
func (c *Client) post(ctx context.Context, path string, body any) (*APIResponse, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("nhanh: rate limit wait: %w", err)
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("nhanh: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s%s?appId=%s&businessId=%s", baseURL, path, c.appID, c.businessID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("nhanh: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nhanh: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("nhanh: read response: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("nhanh: parse response: %w", err)
	}

	if apiResp.Code != 1 {
		msg := string(apiResp.Messages)
		if apiResp.ErrorCode == "ERR_429" {
			return nil, fmt.Errorf("nhanh: rate limit exceeded: %s", msg)
		}
		return nil, fmt.Errorf("nhanh: API error [%s]: %s", apiResp.ErrorCode, msg)
	}

	return &apiResp, nil
}

// ClampPageSize returns a page size within valid bounds.
func ClampPageSize(size, maxSize int) int {
	if size <= 0 {
		return defaultPageSize
	}
	if size > maxSize {
		return maxSize
	}
	return size
}
