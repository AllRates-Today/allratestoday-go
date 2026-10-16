// Package allratestoday is the official Go client for the AllRatesToday
// exchange rate API (https://allratestoday.com). It provides real-time
// mid-market exchange rates for 160+ currencies, conversion, up to a year
// of daily history, and the official rates published by 121 central banks
// and 3 tax authorities.
//
// Create a free API key at https://allratestoday.com/register (300 free
// requests, lifetime). Keys look like "art_live_...", and the client sends
// yours as a Bearer token in the Authorization header. Symbols,
// OfficialRates and OfficialSources work without a key.
//
//	client := allratestoday.NewClient("art_live_your_api_key")
//	rates, err := client.Latest(context.Background(), "USD", []string{"EUR", "GBP"})
package allratestoday

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the production API host.
const DefaultBaseURL = "https://allratestoday.com"

// ErrMissingAPIKey is returned by methods that need an API key when the
// client was created with an empty one.
var ErrMissingAPIKey = errors.New("allratestoday: API key is required; register for free at https://allratestoday.com/register")

// Client is the AllRatesToday API client. Create one with NewClient.
// A Client is safe for concurrent use by multiple goroutines.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	now        func() time.Time
}

// Option configures a Client created by NewClient.
type Option func(*Client)

// WithBaseURL overrides the default API host (useful for testing).
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
// Get a free key at https://allratestoday.com/register. Pass "" to use only
// the keyless methods (Symbols, OfficialRates, OfficialSources).
func NewClient(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		now:        time.Now,
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
	// Message is the response body (usually a JSON error payload such as
	// {"error":"..."}).
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("allratestoday: API error (HTTP %d): %s", e.StatusCode, e.Message)
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

// Rate is one element of the list returned by GET /api/v1/rates.
type Rate struct {
	Rate   float64 `json:"rate"`
	Source string  `json:"source"`
	Target string  `json:"target"`
	// Time is the observation time, e.g. "2026-10-16T11:31:01+0000".
	Time string `json:"time"`
}

// LatestResponse is returned by Latest.
type LatestResponse struct {
	// Deprecated: always true. Failures are returned as an error.
	Success bool   `json:"success"`
	Base    string `json:"base"`
	// Date is the observation time of the rates, e.g.
	// "2026-10-16T11:31:01+0000".
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
}

// ConvertResponse is returned by Convert and ConvertOnDate.
type ConvertResponse struct {
	// Deprecated: always true. Failures are returned as an error.
	Success bool    `json:"success"`
	From    string  `json:"from"`
	To      string  `json:"to"`
	Amount  float64 `json:"amount"`
	Result  float64 `json:"result"`
	Rate    float64 `json:"rate"`
	// Date is set (YYYY-MM-DD) only for ConvertOnDate.
	Date string `json:"date,omitempty"`
}

// HistoricalResponse is returned by Historical.
type HistoricalResponse struct {
	// Deprecated: always true. Failures are returned as an error.
	Success bool               `json:"success"`
	Base    string             `json:"base"`
	Date    string             `json:"date"`
	Rates   map[string]float64 `json:"rates"`
}

// TimeSeriesResponse is returned by TimeSeries. Rates is keyed by date
// (YYYY-MM-DD), then by currency code.
type TimeSeriesResponse struct {
	// Deprecated: always true. Failures are returned as an error.
	Success   bool                          `json:"success"`
	Base      string                        `json:"base"`
	StartDate string                        `json:"start_date"`
	EndDate   string                        `json:"end_date"`
	Rates     map[string]map[string]float64 `json:"rates"`
}

// SymbolsResponse is returned by Symbols.
type SymbolsResponse struct {
	// Deprecated: always true. Failures are returned as an error.
	Success bool `json:"success"`
	// Symbols maps each currency code to its details.
	Symbols map[string]CurrencySymbol `json:"symbols"`
	// Currencies is the list in the order the API returns it.
	Currencies []CurrencySymbol `json:"currencies"`
	Count      int              `json:"count"`
}

// CurrencySymbol describes one supported currency.
type CurrencySymbol struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
}

// RateResponse is returned by GetRate.
type RateResponse struct {
	// Deprecated: always true. Failures are returned as an error.
	Success bool    `json:"success"`
	From    string  `json:"from"`
	To      string  `json:"to"`
	Rate    float64 `json:"rate"`
	// Date is the observation time, e.g. "2026-10-16T11:31:01+0000".
	Date string `json:"date"`
}

// HistoricalRatesResponse is returned by GetHistoricalRates.
type HistoricalRatesResponse struct {
	// Deprecated: always true. Failures are returned as an error.
	Success bool   `json:"success"`
	Source  string `json:"source"`
	Target  string `json:"target"`
	Period  string `json:"period"`
	// Rates is sorted oldest first.
	Rates []HistoricalRatePoint `json:"data"`
}

// HistoricalRatePoint is a single observation in a HistoricalRatesResponse.
type HistoricalRatePoint struct {
	// Date is YYYY-MM-DD.
	Date string  `json:"date"`
	Rate float64 `json:"rate"`
	// Timestamp is milliseconds since the Unix epoch.
	Timestamp int64 `json:"timestamp"`
}

// OfficialRateEntry is one row of an official published table.
type OfficialRateEntry struct {
	Base  string `json:"base"`
	Quote string `json:"quote"`
	// Type is the rate type as the institution labels it, e.g.
	// "reference", "buy", "sell", "mid".
	Type  string  `json:"type"`
	Value float64 `json:"value"`
}

// OfficialRateAttribution is the attribution block on official responses.
type OfficialRateAttribution struct {
	Source string `json:"source"`
	URL    string `json:"url"`
	Terms  string `json:"terms"`
}

// OfficialRatesResponse is returned by OfficialRates. With a pair, Source,
// Target, Rate and RateType are set; without one, Rates holds the full table.
type OfficialRatesResponse struct {
	Bank string `json:"bank"`
	// RateDate is the publication date of the table (YYYY-MM-DD); cite this,
	// not today's date.
	RateDate string              `json:"rate_date"`
	Rates    []OfficialRateEntry `json:"rates,omitempty"`
	Source   string              `json:"source,omitempty"`
	Target   string              `json:"target,omitempty"`
	Rate     float64             `json:"rate,omitempty"`
	RateType string              `json:"rate_type,omitempty"`
	// Derived is true when the pair was cross-computed inside the bank's
	// own table rather than published directly.
	Derived bool   `json:"derived,omitempty"`
	Method  string `json:"method,omitempty"`
	AgeDays int    `json:"age_days"`
	// Stale is true when the institution is behind its own schedule.
	Stale       bool                    `json:"stale"`
	Cadence     string                  `json:"cadence,omitempty"`
	Attribution OfficialRateAttribution `json:"attribution"`
	FreeKey     string                  `json:"free_key,omitempty"`
}

// OfficialSource is one entry returned by OfficialSources.
type OfficialSource struct {
	// Code is what you pass as bank to OfficialRates, e.g. "ecb".
	Code string `json:"code"`
	Name string `json:"name"`
	// Latest is the latest publication date, or "" if never ingested.
	Latest  string `json:"latest"`
	AgeDays int    `json:"age_days"`
	Stale   bool   `json:"stale"`
	Cadence string `json:"cadence"`
}

// OfficialSourcesResponse is returned by OfficialSources.
type OfficialSourcesResponse struct {
	Sources    []OfficialSource `json:"sources"`
	StaleCount int              `json:"stale_count"`
	CheckedAt  string           `json:"checked_at"`
	Docs       string           `json:"docs"`
	FreeKey    string           `json:"free_key"`
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
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
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

func (c *Client) requireKey() error {
	if c.apiKey == "" {
		return ErrMissingAPIKey
	}
	return nil
}

func (c *Client) rates(ctx context.Context, source string, targets []string) ([]Rate, error) {
	if err := c.requireKey(); err != nil {
		return nil, err
	}
	params := url.Values{"source": {source}}
	if len(targets) > 0 {
		params.Set("target", strings.Join(targets, ","))
	}
	var out []Rate
	if err := c.get(ctx, "/api/v1/rates", params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// windowFor picks the smallest history window ending today that reaches
// start. The history endpoint serves fixed windows (7d, 30d, 1y) only.
func (c *Client) windowFor(start string) (string, error) {
	d, err := time.Parse("2006-01-02", start)
	if err != nil {
		return "", fmt.Errorf("allratestoday: invalid date %q, use YYYY-MM-DD", start)
	}
	today, _ := time.Parse("2006-01-02", c.now().UTC().Format("2006-01-02"))
	days := int(today.Sub(d).Hours() / 24)
	switch {
	case days < 0:
		return "", fmt.Errorf("allratestoday: date %s is in the future", start)
	case days <= 6:
		return "7d", nil
	case days <= 29:
		return "30d", nil
	case days <= 364:
		return "1y", nil
	}
	return "", fmt.Errorf("allratestoday: historical market rates cover the last 365 days; %s is older", start)
}

func (c *Client) historyPoints(ctx context.Context, source, target, period string) (*HistoricalRatesResponse, error) {
	if err := c.requireKey(); err != nil {
		return nil, err
	}
	params := url.Values{"source": {source}, "target": {target}, "period": {period}}
	var out HistoricalRatesResponse
	if err := c.get(ctx, "/api/historical-rates", params, &out); err != nil {
		return nil, err
	}
	sort.SliceStable(out.Rates, func(i, j int) bool { return out.Rates[i].Timestamp < out.Rates[j].Timestamp })
	return &out, nil
}

// rateOn returns the rate on date, or the closest earlier point (the 1y
// window is weekly). ok is false when no point is on or before date.
func (c *Client) rateOn(ctx context.Context, source, target, date string) (rate float64, ok bool, err error) {
	period, err := c.windowFor(date)
	if err != nil {
		return 0, false, err
	}
	h, err := c.historyPoints(ctx, source, target, period)
	if err != nil {
		return 0, false, err
	}
	for _, p := range h.Rates {
		if day(p.Date) > date {
			break
		}
		rate, ok = p.Rate, true
	}
	return rate, ok, nil
}

func day(s string) string {
	if len(s) > 10 {
		return s[:10]
	}
	return s
}

// ---------------------------------------------------------------------------
// Public API: market rates (API key required)
// ---------------------------------------------------------------------------

// Latest returns the most recent mid-market rates for a base currency.
// Pass a nil or empty symbols slice to receive every available currency.
func (c *Client) Latest(ctx context.Context, base string, symbols []string) (*LatestResponse, error) {
	list, err := c.rates(ctx, base, symbols)
	if err != nil {
		return nil, err
	}
	out := &LatestResponse{Success: true, Base: base, Rates: make(map[string]float64, len(list))}
	for _, r := range list {
		out.Rates[r.Target] = r.Rate
		if r.Time != "" {
			out.Date = r.Time
		}
	}
	return out, nil
}

// GetRates returns the raw rate list from GET /api/v1/rates, one entry per
// target, each with its observation time. Pass no targets for every
// available currency.
func (c *Client) GetRates(ctx context.Context, source string, targets ...string) ([]Rate, error) {
	return c.rates(ctx, source, targets)
}

// GetRate returns the current rate for a single currency pair.
func (c *Client) GetRate(ctx context.Context, from, to string) (*RateResponse, error) {
	list, err := c.rates(ctx, from, []string{to})
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("allratestoday: no rate returned for %s/%s", from, to)
	}
	r := list[0]
	return &RateResponse{Success: true, From: r.Source, To: r.Target, Rate: r.Rate, Date: r.Time}, nil
}

// Convert converts an amount from one currency to another at the current
// rate.
func (c *Client) Convert(ctx context.Context, from, to string, amount float64) (*ConvertResponse, error) {
	if err := c.requireKey(); err != nil {
		return nil, err
	}
	params := url.Values{
		"source": {from},
		"target": {to},
		"amount": {strconv.FormatFloat(amount, 'f', -1, 64)},
	}
	var raw struct {
		To struct {
			Amount float64 `json:"amount"`
		} `json:"to"`
		Rate float64 `json:"rate"`
	}
	if err := c.get(ctx, "/api/v1/rates", params, &raw); err != nil {
		return nil, err
	}
	return &ConvertResponse{Success: true, From: from, To: to, Amount: amount, Result: raw.To.Amount, Rate: raw.Rate}, nil
}

// ConvertOnDate converts an amount at the rate on a past date (YYYY-MM-DD,
// within the last 365 days). It uses the closest earlier point when the
// date has none (the 1-year window is weekly).
func (c *Client) ConvertOnDate(ctx context.Context, from, to string, amount float64, date string) (*ConvertResponse, error) {
	rate, ok, err := c.rateOn(ctx, from, to, date)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("allratestoday: no %s/%s rate available on or before %s", from, to, date)
	}
	result, _ := strconv.ParseFloat(strconv.FormatFloat(amount*rate, 'f', 6, 64), 64)
	return &ConvertResponse{Success: true, From: from, To: to, Amount: amount, Result: result, Rate: rate, Date: date}, nil
}

// Historical returns rates as of a past date (YYYY-MM-DD, within the last
// 365 days) for the given symbols. symbols is required: the history
// endpoint takes one target per call, so each symbol costs one request.
func (c *Client) Historical(ctx context.Context, date, base string, symbols []string) (*HistoricalResponse, error) {
	if len(symbols) == 0 {
		return nil, errors.New("allratestoday: Historical needs symbols, e.g. []string{\"EUR\"}")
	}
	out := &HistoricalResponse{Success: true, Base: base, Date: date, Rates: map[string]float64{}}
	for _, target := range symbols {
		rate, ok, err := c.rateOn(ctx, base, target, date)
		if err != nil {
			return nil, err
		}
		if ok {
			out.Rates[target] = rate
		}
	}
	return out, nil
}

// TimeSeries returns daily rates between two dates (YYYY-MM-DD, start
// within the last 365 days), keyed by date then currency. symbols is
// required; each symbol costs one request.
func (c *Client) TimeSeries(ctx context.Context, startDate, endDate, base string, symbols []string) (*TimeSeriesResponse, error) {
	if len(symbols) == 0 {
		return nil, errors.New("allratestoday: TimeSeries needs symbols, e.g. []string{\"EUR\"}")
	}
	period, err := c.windowFor(startDate)
	if err != nil {
		return nil, err
	}
	out := &TimeSeriesResponse{Success: true, Base: base, StartDate: startDate, EndDate: endDate, Rates: map[string]map[string]float64{}}
	for _, target := range symbols {
		h, err := c.historyPoints(ctx, base, target, period)
		if err != nil {
			return nil, err
		}
		for _, p := range h.Rates {
			d := day(p.Date)
			if d < startDate || d > endDate {
				continue
			}
			if out.Rates[d] == nil {
				out.Rates[d] = map[string]float64{}
			}
			out.Rates[d][target] = p.Rate
		}
	}
	return out, nil
}

// GetHistoricalRates returns a currency pair's history over a preset
// period: "1d", "7d", "30d" or "1y" (daily points; weekly for "1y").
// Points are sorted oldest first.
func (c *Client) GetHistoricalRates(ctx context.Context, source, target, period string) (*HistoricalRatesResponse, error) {
	h, err := c.historyPoints(ctx, source, target, period)
	if err != nil {
		return nil, err
	}
	h.Success = true
	if h.Source == "" {
		h.Source = source
	}
	if h.Target == "" {
		h.Target = target
	}
	if h.Period == "" {
		h.Period = period
	}
	return h, nil
}

// ---------------------------------------------------------------------------
// Public API: keyless
// ---------------------------------------------------------------------------

// Symbols lists every supported currency. No API key needed.
func (c *Client) Symbols(ctx context.Context) (*SymbolsResponse, error) {
	var out SymbolsResponse
	if err := c.get(ctx, "/api/v1/symbols", nil, &out); err != nil {
		return nil, err
	}
	out.Success = true
	out.Symbols = make(map[string]CurrencySymbol, len(out.Currencies))
	for _, cur := range out.Currencies {
		out.Symbols[cur.Code] = cur
	}
	if out.Count == 0 {
		out.Count = len(out.Currencies)
	}
	return &out, nil
}

var bankCode = regexp.MustCompile(`^[a-z0-9_-]{2,20}$`)

// OfficialRates returns the latest table of official rates published by a
// central bank or tax authority (e.g. "ecb", "fed", "hmrc"). Pass source
// and target (e.g. "EUR", "USD") for one pair, or both empty for the full
// table. No API key needed.
func (c *Client) OfficialRates(ctx context.Context, bank, source, target string) (*OfficialRatesResponse, error) {
	code := strings.ToLower(strings.TrimSpace(bank))
	if !bankCode.MatchString(code) {
		return nil, fmt.Errorf("allratestoday: invalid source code %q; call OfficialSources for the list", bank)
	}
	if (source == "") != (target == "") {
		return nil, errors.New("allratestoday: provide both source and target, or neither")
	}
	params := url.Values{}
	if source != "" {
		params.Set("source", strings.ToUpper(source))
		params.Set("target", strings.ToUpper(target))
	}
	var out OfficialRatesResponse
	if err := c.get(ctx, "/api/open/central-bank/"+url.PathEscape(code), params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OfficialSources lists every covered official source with its code, name,
// latest publication date and whether it is behind schedule. No API key
// needed.
func (c *Client) OfficialSources(ctx context.Context) (*OfficialSourcesResponse, error) {
	var out OfficialSourcesResponse
	if err := c.get(ctx, "/api/open/central-banks", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
