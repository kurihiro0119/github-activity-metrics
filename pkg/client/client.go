package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/kurihiro0119/github-activity-metrics/internal/domain"
)

// Client is the API client for github-activity-metrics
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new API client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetOrgMetrics retrieves organization-level metrics
func (c *Client) GetOrgMetrics(org string, start, end time.Time, granularity string) (*domain.OrgMetrics, error) {
	path := fmt.Sprintf("/api/v1/orgs/%s/metrics", org)
	params := c.buildTimeParams(start, end, granularity)

	var response struct {
		Data *domain.OrgMetrics `json:"data"`
	}
	if err := c.get(path, params, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

// GetMemberMetrics retrieves member-level metrics
func (c *Client) GetMemberMetrics(org, member string, start, end time.Time, granularity string) (*domain.MemberMetrics, error) {
	path := fmt.Sprintf("/api/v1/orgs/%s/members/%s/metrics", org, member)
	params := c.buildTimeParams(start, end, granularity)

	var response struct {
		Data *domain.MemberMetrics `json:"data"`
	}
	if err := c.get(path, params, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

// GetRepoMetrics retrieves repository-level metrics
func (c *Client) GetRepoMetrics(org, repo string, start, end time.Time, granularity string) (*domain.RepoMetrics, error) {
	path := fmt.Sprintf("/api/v1/orgs/%s/repos/%s/metrics", org, repo)
	params := c.buildTimeParams(start, end, granularity)

	var response struct {
		Data *domain.RepoMetrics `json:"data"`
	}
	if err := c.get(path, params, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

// GetMembersMetrics retrieves metrics for all members
func (c *Client) GetMembersMetrics(org string, start, end time.Time, granularity string) ([]*domain.MemberMetrics, error) {
	path := fmt.Sprintf("/api/v1/orgs/%s/members/metrics", org)
	params := c.buildTimeParams(start, end, granularity)

	var response struct {
		Data []*domain.MemberMetrics `json:"data"`
	}
	if err := c.get(path, params, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

// GetReposMetrics retrieves metrics for all repositories
func (c *Client) GetReposMetrics(org string, start, end time.Time, granularity string) ([]*domain.RepoMetrics, error) {
	path := fmt.Sprintf("/api/v1/orgs/%s/repos/metrics", org)
	params := c.buildTimeParams(start, end, granularity)

	var response struct {
		Data []*domain.RepoMetrics `json:"data"`
	}
	if err := c.get(path, params, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

// GetTimeSeriesMetrics retrieves time series metrics
func (c *Client) GetTimeSeriesMetrics(org string, metricType string, start, end time.Time, granularity string) (*domain.TimeSeriesData, error) {
	path := fmt.Sprintf("/api/v1/orgs/%s/metrics/timeseries", org)
	params := c.buildTimeParams(start, end, granularity)
	params.Set("type", metricType)

	var response struct {
		Data *domain.TimeSeriesData `json:"data"`
	}
	if err := c.get(path, params, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

// HealthCheck checks if the API is healthy
func (c *Client) HealthCheck() error {
	var response struct {
		Status string `json:"status"`
	}
	if err := c.get("/health", nil, &response); err != nil {
		return err
	}
	if response.Status != "ok" {
		return fmt.Errorf("unhealthy status: %s", response.Status)
	}
	return nil
}

func (c *Client) buildTimeParams(start, end time.Time, granularity string) url.Values {
	params := url.Values{}
	if !start.IsZero() {
		params.Set("start", start.Format("2006-01-02"))
	}
	if !end.IsZero() {
		params.Set("end", end.Format("2006-01-02"))
	}
	if granularity != "" {
		params.Set("granularity", granularity)
	}
	return params
}

func (c *Client) get(path string, params url.Values, result interface{}) error {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return err
	}
	if params != nil {
		u.RawQuery = params.Encode()
	}

	resp, err := c.httpClient.Get(u.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(result)
}
