package allratestoday

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// newTestServer returns an httptest server that asserts the request path,
// query and Authorization header, then responds with the given JSON body.
func newTestServer(t *testing.T, wantPath string, wantQuery map[string]string, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != wantPath {
			t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer art_live_test_key" {
			t.Errorf("Authorization = %q, want Bearer art_live_test_key", got)
		}
		q := r.URL.Query()
		for k, want := range wantQuery {
			if got := q.Get(k); got != want {
				t.Errorf("query %s = %q, want %q", k, got, want)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
}

func newTestClient(server *httptest.Server) *Client {
	return NewClient("art_live_test_key", WithBaseURL(server.URL))
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
	server := newTestServer(t, "/v1/latest",
		map[string]string{"base": "USD", "symbols": "EUR,GBP"},
		`{"success":true,"base":"USD","date":"2026-08-01","rates":{"EUR":0.92,"GBP":0.79}}`)
	defer server.Close()

	resp, err := newTestClient(server).Latest(context.Background(), "USD", []string{"EUR", "GBP"})
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if !resp.Success || resp.Base != "USD" || resp.Rates["EUR"] != 0.92 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestLatestAllSymbols(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("symbols") {
			t.Error("symbols param should be omitted when nil")
		}
		w.Write([]byte(`{"success":true,"base":"USD","rates":{}}`))
	}))
	defer server.Close()

	if _, err := newTestClient(server).Latest(context.Background(), "USD", nil); err != nil {
		t.Fatalf("Latest: %v", err)
	}
}

func TestConvert(t *testing.T) {
	server := newTestServer(t, "/v1/convert",
		map[string]string{"from": "USD", "to": "EUR", "amount": "100"},
		`{"success":true,"from":"USD","to":"EUR","amount":100,"result":92,"rate":0.92}`)
	defer server.Close()

	resp, err := newTestClient(server).Convert(context.Background(), "USD", "EUR", 100)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if resp.Result != 92 || resp.Rate != 0.92 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestHistorical(t *testing.T) {
	server := newTestServer(t, "/v1/historical",
		map[string]string{"date": "2025-01-15", "base": "USD", "symbols": "EUR"},
		`{"success":true,"base":"USD","date":"2025-01-15","rates":{"EUR":0.91}}`)
	defer server.Close()

	resp, err := newTestClient(server).Historical(context.Background(), "2025-01-15", "USD", []string{"EUR"})
	if err != nil {
		t.Fatalf("Historical: %v", err)
	}
	if resp.Date != "2025-01-15" || resp.Rates["EUR"] != 0.91 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestTimeSeries(t *testing.T) {
	server := newTestServer(t, "/v1/timeseries",
		map[string]string{"start_date": "2025-01-01", "end_date": "2025-01-03", "base": "USD", "symbols": "EUR"},
		`{"success":true,"base":"USD","start_date":"2025-01-01","end_date":"2025-01-03",
		  "rates":{"2025-01-01":{"EUR":0.91},"2025-01-02":{"EUR":0.92},"2025-01-03":{"EUR":0.93}}}`)
	defer server.Close()

	resp, err := newTestClient(server).TimeSeries(context.Background(), "2025-01-01", "2025-01-03", "USD", []string{"EUR"})
	if err != nil {
		t.Fatalf("TimeSeries: %v", err)
	}
	if len(resp.Rates) != 3 || resp.Rates["2025-01-02"]["EUR"] != 0.92 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestSymbols(t *testing.T) {
	server := newTestServer(t, "/v1/symbols", nil,
		`{"success":true,"symbols":{"USD":{"code":"USD","name":"United States Dollar"},"EUR":{"code":"EUR","name":"Euro"}}}`)
	defer server.Close()

	resp, err := newTestClient(server).Symbols(context.Background())
	if err != nil {
		t.Fatalf("Symbols: %v", err)
	}
	if resp.Symbols["EUR"].Name != "Euro" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestGetRate(t *testing.T) {
	server := newTestServer(t, "/v1/rate",
		map[string]string{"from": "USD", "to": "EUR"},
		`{"success":true,"from":"USD","to":"EUR","rate":0.92,"date":"2026-08-01"}`)
	defer server.Close()

	resp, err := newTestClient(server).GetRate(context.Background(), "USD", "EUR")
	if err != nil {
		t.Fatalf("GetRate: %v", err)
	}
	if resp.Rate != 0.92 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestGetHistoricalRates(t *testing.T) {
	server := newTestServer(t, "/v1/historical-rates",
		map[string]string{"source": "USD", "target": "EUR", "period": "7d"},
		`{"success":true,"source":"USD","target":"EUR","period":"7d",
		  "rates":[{"date":"2025-05-25","rate":0.91},{"date":"2025-05-26","rate":0.92}]}`)
	defer server.Close()

	resp, err := newTestClient(server).GetHistoricalRates(context.Background(), "USD", "EUR", "7d")
	if err != nil {
		t.Fatalf("GetHistoricalRates: %v", err)
	}
	if len(resp.Rates) != 2 || resp.Rates[1].Rate != 0.92 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer server.Close()

	_, err := newTestClient(server).Latest(context.Background(), "USD", nil)
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
	}
	if apiErr.Message != `{"error":"invalid api key"}` {
		t.Errorf("Message = %q", apiErr.Message)
	}
}

func TestParseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	_, err := newTestClient(server).Latest(context.Background(), "USD", nil)
	if err == nil {
		t.Fatal("expected a parse error, got nil")
	}
	var apiErr *Error
	if errors.As(err, &apiErr) {
		t.Errorf("parse failure should not be an *Error: %v", err)
	}
}

func TestContextCancellation(t *testing.T) {
	server := newTestServer(t, "/v1/symbols", nil, `{"success":true}`)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := newTestClient(server).Symbols(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// TestLiveSmoke hits the real API. It is skipped unless ALLRATES_API_KEY
// is set in the environment.
func TestLiveSmoke(t *testing.T) {
	apiKey := os.Getenv("ALLRATES_API_KEY")
	if apiKey == "" {
		t.Skip("ALLRATES_API_KEY not set; skipping live smoke test")
	}
	client := NewClient(apiKey)
	resp, err := client.Latest(context.Background(), "USD", []string{"EUR", "GBP"})
	if err != nil {
		t.Fatalf("live Latest: %v", err)
	}
	if resp.Rates["EUR"] <= 0 {
		t.Errorf("expected a positive EUR rate, got %+v", resp)
	}
}
