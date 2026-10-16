package allratestoday

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// Response bodies captured from the live API (trimmed).
const (
	ratesBody   = `[{"rate":0.888455,"source":"USD","target":"EUR","time":"2026-10-16T12:04:01+0000"},{"rate":0.749738,"source":"USD","target":"GBP","time":"2026-10-16T12:04:01+0000"}]`
	rateBody    = `[{"rate":0.888455,"source":"USD","target":"EUR","time":"2026-10-16T12:04:01+0000"}]`
	convertBody = `{"from":{"currency":"USD","amount":100},"to":{"currency":"EUR","amount":88.8455},"rate":0.888455,"source":"interbank"}`
	historyEUR  = `{"source":"USD","target":"EUR","data":[{"date":"2026-10-14","rate":0.861,"timestamp":1791936000000},{"date":"2026-10-12","rate":0.859,"timestamp":1791763200000},{"date":"2026-10-15","rate":0.862,"timestamp":1792022400000},{"date":"2026-10-16","rate":0.888455,"timestamp":1792108800000}],"source_api":"interbank","period":"7d"}`
	historyGBP  = `{"source":"USD","target":"GBP","data":[{"date":"2026-10-12","rate":0.741,"timestamp":1791763200000},{"date":"2026-10-15","rate":0.745,"timestamp":1792022400000}],"source_api":"interbank","period":"7d"}`
	symbolsBody = `{"currencies":[{"code":"USD","name":"US Dollar","symbol":"$"},{"code":"EUR","name":"Euro","symbol":"€"}],"count":2}`
	officialEUR = `{"bank":"ecb","rate_date":"2026-10-15","source":"EUR","target":"USD","rate":1.1208,"rate_type":"reference","derived":false,"method":"published","age_days":1,"stale":false,"cadence":"daily","attribution":{"source":"Official rates published by ECB, served by AllRatesToday","url":"https://allratestoday.com/central-bank-rates-api/ecb/","terms":"Free to use with visible attribution linking to allratestoday.com"},"free_key":"..."}`
	sourcesBody = `{"sources":[{"code":"amcm","name":"Monetary Authority of Macao","latest":"2026-10-16","age_days":0,"stale":false,"cadence":"daily"}],"stale_count":3,"checked_at":"2026-10-16T12:04:27.421Z","docs":"https://allratestoday.com/central-bank-rates-api/","free_key":"..."}`
)

type route struct {
	query map[string]string // expected query params
	body  string
}

// newMock serves the given routes keyed by path and records every request.
// When keyed is true it asserts the Bearer header; otherwise it asserts the
// header is absent.
func newMock(t *testing.T, keyed bool, routes map[string][]route) (*httptest.Server, *[]*http.Request) {
	t.Helper()
	var seen []*http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r)
		auth := r.Header.Get("Authorization")
		if keyed && auth != "Bearer art_live_test_key" {
			t.Errorf("Authorization = %q, want Bearer art_live_test_key", auth)
		}
		if !keyed && auth != "" {
			t.Errorf("keyless call sent Authorization %q", auth)
		}
		q := r.URL.Query()
	next:
		for _, rt := range routes[r.URL.Path] {
			for k, v := range rt.query {
				if q.Get(k) != v {
					continue next
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(rt.body))
			return
		}
		t.Errorf("unexpected request %s", r.URL.String())
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"Not found"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func testClient(srv *httptest.Server, key string) *Client {
	c := NewClient(key, WithBaseURL(srv.URL))
	c.now = func() time.Time { return time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC) }
	return c
}

func TestNewClientDefaults(t *testing.T) {
	c := NewClient("art_live_test_key")
	if c.baseURL != DefaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, DefaultBaseURL)
	}
	if c.httpClient == nil {
		t.Fatal("httpClient is nil")
	}
}

func TestOptions(t *testing.T) {
	hc := &http.Client{}
	c := NewClient("k", WithBaseURL("https://example.com/"), WithHTTPClient(hc))
	if c.baseURL != "https://example.com" {
		t.Errorf("baseURL = %q, want trailing slash trimmed", c.baseURL)
	}
	if c.httpClient != hc {
		t.Error("WithHTTPClient did not replace the http client")
	}
}

func TestLatest(t *testing.T) {
	srv, _ := newMock(t, true, map[string][]route{
		"/api/v1/rates": {{query: map[string]string{"source": "USD", "target": "EUR,GBP"}, body: ratesBody}},
	})
	resp, err := testClient(srv, "art_live_test_key").Latest(context.Background(), "USD", []string{"EUR", "GBP"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Base != "USD" || resp.Rates["EUR"] != 0.888455 || resp.Rates["GBP"] != 0.749738 {
		t.Errorf("unexpected response: %+v", resp)
	}
	if resp.Date != "2026-10-16T12:04:01+0000" {
		t.Errorf("Date = %q", resp.Date)
	}
}

func TestLatestAllSymbols(t *testing.T) {
	srv, seen := newMock(t, true, map[string][]route{
		"/api/v1/rates": {{query: map[string]string{"source": "EUR"}, body: ratesBody}},
	})
	if _, err := testClient(srv, "art_live_test_key").Latest(context.Background(), "EUR", nil); err != nil {
		t.Fatal(err)
	}
	if (*seen)[0].URL.Query().Has("target") {
		t.Error("target should be omitted when symbols is nil")
	}
}

func TestGetRatesAndGetRate(t *testing.T) {
	srv, _ := newMock(t, true, map[string][]route{
		"/api/v1/rates": {
			{query: map[string]string{"source": "USD", "target": "EUR,GBP"}, body: ratesBody},
			{query: map[string]string{"source": "USD", "target": "EUR"}, body: rateBody},
		},
	})
	c := testClient(srv, "art_live_test_key")
	list, err := c.GetRates(context.Background(), "USD", "EUR", "GBP")
	if err != nil || len(list) != 2 || list[1].Target != "GBP" || list[1].Time == "" {
		t.Fatalf("GetRates = %+v, %v", list, err)
	}
	r, err := c.GetRate(context.Background(), "USD", "EUR")
	if err != nil {
		t.Fatal(err)
	}
	if r.From != "USD" || r.To != "EUR" || r.Rate != 0.888455 || r.Date != "2026-10-16T12:04:01+0000" {
		t.Errorf("GetRate = %+v", r)
	}
}

func TestConvert(t *testing.T) {
	srv, _ := newMock(t, true, map[string][]route{
		"/api/v1/rates": {{query: map[string]string{"source": "USD", "target": "EUR", "amount": "100"}, body: convertBody}},
	})
	resp, err := testClient(srv, "art_live_test_key").Convert(context.Background(), "USD", "EUR", 100)
	if err != nil {
		t.Fatal(err)
	}
	if resp.From != "USD" || resp.To != "EUR" || resp.Amount != 100 || resp.Result != 88.8455 || resp.Rate != 0.888455 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestConvertOnDate(t *testing.T) {
	srv, _ := newMock(t, true, map[string][]route{
		"/api/historical-rates": {{query: map[string]string{"source": "USD", "target": "EUR", "period": "7d"}, body: historyEUR}},
	})
	// 2026-10-13 has no point: the closest earlier one (10-12) is used.
	resp, err := testClient(srv, "art_live_test_key").ConvertOnDate(context.Background(), "USD", "EUR", 1000, "2026-10-13")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Rate != 0.859 || resp.Result != 859 || resp.Date != "2026-10-13" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestHistorical(t *testing.T) {
	srv, seen := newMock(t, true, map[string][]route{
		"/api/historical-rates": {
			{query: map[string]string{"target": "EUR", "period": "7d"}, body: historyEUR},
			{query: map[string]string{"target": "GBP", "period": "7d"}, body: historyGBP},
		},
	})
	resp, err := testClient(srv, "art_live_test_key").Historical(context.Background(), "2026-10-14", "USD", []string{"EUR", "GBP"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Date != "2026-10-14" || resp.Rates["EUR"] != 0.861 || resp.Rates["GBP"] != 0.741 {
		t.Errorf("unexpected response: %+v", resp)
	}
	if len(*seen) != 2 {
		t.Errorf("requests = %d, want one per symbol", len(*seen))
	}
}

func TestHistoricalValidation(t *testing.T) {
	c := NewClient("art_live_test_key", WithBaseURL("http://127.0.0.1:1"))
	c.now = func() time.Time { return time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC) }
	ctx := context.Background()
	if _, err := c.Historical(ctx, "2026-10-01", "USD", nil); err == nil {
		t.Error("expected error for missing symbols")
	}
	for _, d := range []string{"2025-01-15", "2026-10-17", "15/01/2026"} {
		if _, err := c.Historical(ctx, d, "USD", []string{"EUR"}); err == nil {
			t.Errorf("expected error for date %s", d)
		}
	}
}

func TestWindowFor(t *testing.T) {
	c := NewClient("k")
	c.now = func() time.Time { return time.Date(2026, 10, 16, 23, 0, 0, 0, time.UTC) }
	for date, want := range map[string]string{"2026-10-16": "7d", "2026-10-10": "7d", "2026-10-09": "30d", "2026-09-17": "30d", "2026-09-16": "1y", "2025-10-17": "1y"} {
		if got, err := c.windowFor(date); err != nil || got != want {
			t.Errorf("windowFor(%s) = %q, %v; want %q", date, got, err, want)
		}
	}
}

func TestTimeSeries(t *testing.T) {
	srv, _ := newMock(t, true, map[string][]route{
		"/api/historical-rates": {
			{query: map[string]string{"target": "EUR", "period": "7d"}, body: historyEUR},
			{query: map[string]string{"target": "GBP", "period": "7d"}, body: historyGBP},
		},
	})
	resp, err := testClient(srv, "art_live_test_key").TimeSeries(context.Background(), "2026-10-12", "2026-10-15", "USD", []string{"EUR", "GBP"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Rates) != 3 {
		t.Errorf("days = %d, want 3 (10-16 is outside the range): %+v", len(resp.Rates), resp.Rates)
	}
	if resp.Rates["2026-10-15"]["EUR"] != 0.862 || resp.Rates["2026-10-15"]["GBP"] != 0.745 {
		t.Errorf("unexpected 2026-10-15: %+v", resp.Rates["2026-10-15"])
	}
	if resp.StartDate != "2026-10-12" || resp.EndDate != "2026-10-15" {
		t.Errorf("unexpected range: %+v", resp)
	}
}

func TestGetHistoricalRates(t *testing.T) {
	srv, _ := newMock(t, true, map[string][]route{
		"/api/historical-rates": {{query: map[string]string{"source": "USD", "target": "EUR", "period": "7d"}, body: historyEUR}},
	})
	resp, err := testClient(srv, "art_live_test_key").GetHistoricalRates(context.Background(), "USD", "EUR", "7d")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Period != "7d" || len(resp.Rates) != 4 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Rates[0].Date != "2026-10-12" || resp.Rates[3].Rate != 0.888455 || resp.Rates[3].Timestamp != 1792108800000 {
		t.Errorf("points not sorted oldest first: %+v", resp.Rates)
	}
}

func TestSymbolsKeyless(t *testing.T) {
	srv, _ := newMock(t, false, map[string][]route{"/api/v1/symbols": {{body: symbolsBody}}})
	resp, err := testClient(srv, "").Symbols(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Count != 2 || len(resp.Currencies) != 2 || resp.Symbols["EUR"].Name != "Euro" || resp.Symbols["USD"].Symbol != "$" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestOfficialRatesKeyless(t *testing.T) {
	srv, _ := newMock(t, false, map[string][]route{
		"/api/open/central-bank/ecb": {{query: map[string]string{"source": "EUR", "target": "USD"}, body: officialEUR}},
		"/api/open/central-banks":    {{body: sourcesBody}},
	})
	c := testClient(srv, "")
	r, err := c.OfficialRates(context.Background(), "ECB", "eur", "usd")
	if err != nil {
		t.Fatal(err)
	}
	if r.Bank != "ecb" || r.RateDate != "2026-10-15" || r.Rate != 1.1208 || r.RateType != "reference" || r.Attribution.URL == "" {
		t.Errorf("unexpected response: %+v", r)
	}
	s, err := c.OfficialSources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Sources) != 1 || s.Sources[0].Code != "amcm" || s.StaleCount != 3 {
		t.Errorf("unexpected sources: %+v", s)
	}
	if _, err := c.OfficialRates(context.Background(), "ecb", "EUR", ""); err == nil {
		t.Error("expected error when only source is given")
	}
	if _, err := c.OfficialRates(context.Background(), "../x", "", ""); err == nil {
		t.Error("expected error for invalid bank code")
	}
}

func TestMissingKey(t *testing.T) {
	c := NewClient("", WithBaseURL("http://127.0.0.1:1"))
	if _, err := c.Latest(context.Background(), "USD", nil); !errors.Is(err, ErrMissingAPIKey) {
		t.Errorf("expected ErrMissingAPIKey, got %v", err)
	}
}

func TestAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"Authentication required. Provide an API key via Authorization header or log in."}`))
	}))
	defer server.Close()

	_, err := NewClient("art_live_bad", WithBaseURL(server.URL)).Latest(context.Background(), "USD", nil)
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 401 || !strings.Contains(apiErr.Message, "Authentication required") {
		t.Errorf("unexpected error: %+v", apiErr)
	}
}

func TestParseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	_, err := NewClient("k", WithBaseURL(server.URL)).Latest(context.Background(), "USD", nil)
	if err == nil {
		t.Fatal("expected a parse error, got nil")
	}
	var apiErr *Error
	if errors.As(err, &apiErr) {
		t.Errorf("parse failure should not be an *Error: %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	srv, _ := newMock(t, false, map[string][]route{"/api/v1/symbols": {{body: symbolsBody}}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := testClient(srv, "").Symbols(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// TestLiveSmoke hits the real API. It is skipped unless ALLRATES_API_KEY
// is set in the environment. It makes 4 keyed requests.
func TestLiveSmoke(t *testing.T) {
	apiKey := os.Getenv("ALLRATES_API_KEY")
	if apiKey == "" {
		t.Skip("ALLRATES_API_KEY not set; skipping live smoke test")
	}
	ctx := context.Background()
	client := NewClient(apiKey)
	latest, err := client.Latest(ctx, "USD", []string{"EUR", "GBP"})
	if err != nil || latest.Rates["EUR"] <= 0 || latest.Rates["GBP"] <= 0 {
		t.Fatalf("live Latest: %+v, %v", latest, err)
	}
	conv, err := client.Convert(ctx, "USD", "EUR", 100)
	if err != nil || conv.Result <= 0 || conv.Rate <= 0 {
		t.Fatalf("live Convert: %+v, %v", conv, err)
	}
	day := time.Now().UTC().AddDate(0, 0, -3).Format("2006-01-02")
	hist, err := client.Historical(ctx, day, "USD", []string{"EUR"})
	if err != nil || hist.Rates["EUR"] <= 0 {
		t.Fatalf("live Historical(%s): %+v, %v", day, hist, err)
	}
	series, err := client.GetHistoricalRates(ctx, "USD", "EUR", "7d")
	if err != nil || len(series.Rates) == 0 {
		t.Fatalf("live GetHistoricalRates: %+v, %v", series, err)
	}
	sym, err := NewClient("").Symbols(ctx)
	if err != nil || sym.Count < 150 {
		t.Fatalf("live Symbols: %v (count %d)", err, sym.Count)
	}
	off, err := NewClient("").OfficialRates(ctx, "ecb", "EUR", "USD")
	if err != nil || off.Rate <= 0 || off.RateDate == "" {
		t.Fatalf("live OfficialRates: %+v, %v", off, err)
	}
	t.Logf("USD/EUR %v (%s); 100 USD = %v EUR; %s EUR %v; %d history points; %d symbols; ECB %s EUR/USD %v",
		latest.Rates["EUR"], latest.Date, conv.Result, day, hist.Rates["EUR"], len(series.Rates), sym.Count, off.RateDate, off.Rate)
}
