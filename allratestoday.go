// Package allratestoday is the official Go client for the AllRatesToday
// exchange rate API (https://allratestoday.com). It provides access to
// real-time mid-market exchange rates for 160+ currencies, plus historical
// data, time series and conversion.
//
// Create a free API key at https://allratestoday.com/register. Keys look
// like "art_live_...", and the client sends yours as a Bearer token in the
// Authorization header on every request.
//
//	client := allratestoday.NewClient("art_live_your_api_key")
//	rates, err := client.Latest(context.Background(), "USD", []string{"EUR", "GBP"})
package allratestoday

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the production API host.
const DefaultBaseURL = "https://allratestoday.com"

// Client is the AllRatesToday API client. Create one with NewClient.
// A Client is safe for concurrent use by multiple goroutines.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// Option configures a Client created by NewClient.
type Option func(*Client)

// WithBaseURL overrides the default API host (useful for testing or a
// self-hosted deployment).
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// WithHTTPClient replaces the underlying *http.Client (custom timeouts,
// proxies, instrumentation, ...).
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// NewClient returns a Client that authenticates with the given API key.
// Get a free key at https://allratestoday.com/register.
func NewClient(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Error is returned when the API responds with a non-2xx status code.
type Error struct {
	// StatusCode is the HTTP status code returned by the API.
	StatusCode int
	// Message is the response body (usually a JSON error payload).
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("allratestoday: API error (HTTP %d): %s", e.StatusCode, e.Message)
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

// LatestResponse is the response from GET /v1/latest.
type LatestResponse struct {
	Success bool               `json:"success"`
	Base    string             `json:"base"`
	Date    string             `json:"date"`
	Rates   map[string]float64 `json:"rates"`
}

// ConvertResponse is the response from GET /v1/convert.
type ConvertResponse struct {
	Success bool    `json:"success"`
	From    string  `json:"from"`
	To      string  `json:"to"`
	Amount  float64 `json:"amount"`
	Result  float64 `json:"result"`
	Rate    float64 `json:"rate"`
}

// HistoricalResponse is the response from GET /v1/historical.
type HistoricalResponse struct {
	Success bool               `json:"success"`
	Base    string             `json:"base"`
	Date    string             `json:"date"`
	Rates   map[string]float64 `json:"rates"`
}

// TimeSeriesResponse is the response from GET /v1/timeseries. Rates is
// keyed by date (YYYY-MM-DD), then by currency code.
type TimeSeriesResponse struct {
	Success   bool                          `json:"success"`
	Base      string                        `json:"base"`
	StartDate string                        `json:"start_date"`
	EndDate   string                        `json:"end_date"`
	Rates     map[string]map[string]float64 `json:"rates"`
}

// SymbolsResponse is the response from GET /v1/symbols.
type SymbolsResponse struct {
	Success bool                      `json:"success"`
	Symbols map[string]CurrencySymbol `json:"symbols"`
}

// CurrencySymbol describes one supported currency.
type CurrencySymbol struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// RateResponse is the response from GET /v1/rate (single pair).
type RateResponse struct {
	Success bool    `json:"success"`
	From    string  `json:"from"`
	To      string  `json:"to"`
	Rate    float64 `json:"rate"`
	Date    string  `json:"date"`
}

// HistoricalRatesResponse is the response from GET /v1/historical-rates
// (a currency pair over a preset period).
type HistoricalRatesResponse struct {
	Success bool                  `json:"success"`
	Source  string                `json:"source"`
	Target  string                `json:"target"`
	Period  string                `json:"period"`
	Rates   []HistoricalRatePoint `json:"rates"`
}

// HistoricalRatePoint is a single date/rate observation in a
// HistoricalRatesResponse.
type HistoricalRatePoint struct {
	Date string  `json:"date"`
	Rate float64 `json:"rate"`
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (c *Client) get(ctx context.Context, path string, params url.Values, out any) error {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("allratestoday: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("allratestoday: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("allratestoday: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &Error{StatusCode: resp.StatusCode, Message: strings.TrimSpace(string(body))}
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("allratestoday: parse response: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// Latest returns the most recent exchange rates for a base currency.
// Pass a nil or empty symbols slice to receive every available currency.
func (c *Client) Latest(ctx context.Context, base string, symbols []string) (*LatestResponse, error) {
	params := url.Values{"base": {base}}
	if len(symbols) > 0 {
		params.Set("symbols", strings.Join(symbols, ","))
	}
	var out LatestResponse
	if err := c.get(ctx, "/v1/latest", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Convert converts an amount from one currency to another.
func (c *Client) Convert(ctx context.Context, from, to string, amount float64) (*ConvertResponse, error) {
	params := url.Values{
		"from":   {from},
		"to":     {to},
		"amount": {strconv.FormatFloat(amount, 'f', -1, 64)},
	}
	var out ConvertResponse
	if err := c.get(ctx, "/v1/convert", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Historical returns exchange rates as of a specific past date
// (YYYY-MM-DD). Pass a nil or empty symbols slice for every currency.
func (c *Client) Historical(ctx context.Context, date, base string, symbols []string) (*HistoricalResponse, error) {
	params := url.Values{"date": {date}, "base": {base}}
	if len(symbols) > 0 {
		params.Set("symbols", strings.Join(symbols, ","))
	}
	var out HistoricalResponse
	if err := c.get(ctx, "/v1/historical", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// TimeSeries returns exchange rates across a date range, keyed by date.
// Dates are YYYY-MM-DD. Pass a nil or empty symbols slice for every
// currency.
func (c *Client) TimeSeries(ctx context.Context, startDate, endDate, base string, symbols []string) (*TimeSeriesResponse, error) {
	params := url.Values{
		"start_date": {startDate},
		"end_date":   {endDate},
		"base":       {base},
	}
	if len(symbols) > 0 {
		params.Set("symbols", strings.Join(symbols, ","))
	}
	var out TimeSeriesResponse
	if err := c.get(ctx, "/v1/timeseries", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Symbols lists every supported currency code with its details.
func (c *Client) Symbols(ctx context.Context) (*SymbolsResponse, error) {
	var out SymbolsResponse
	if err := c.get(ctx, "/v1/symbols", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRate returns the exchange rate for a single currency pair.
func (c *Client) GetRate(ctx context.Context, from, to string) (*RateResponse, error) {
	params := url.Values{"from": {from}, "to": {to}}
	var out RateResponse
	if err := c.get(ctx, "/v1/rate", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetHistoricalRates returns a currency pair's history over a preset
// period: "1d", "7d", "30d" or "1y".
func (c *Client) GetHistoricalRates(ctx context.Context, source, target, period string) (*HistoricalRatesResponse, error) {
	params := url.Values{"source": {source}, "target": {target}, "period": {period}}
	var out HistoricalRatesResponse
	if err := c.get(ctx, "/v1/historical-rates", params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
