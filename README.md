# AllRatesToday Go SDK — allratestoday-go

The official Go client for the [AllRatesToday](https://allratestoday.com) currency API. A small, zero-dependency wrapper over `net/http` for Go services and CLI tools that need real-time, historical or official central-bank exchange rates without hand-rolling request and response types.

[![Go Reference](https://pkg.go.dev/badge/github.com/AllRates-Today/allratestoday-go.svg)](https://pkg.go.dev/github.com/AllRates-Today/allratestoday-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/AllRates-Today/allratestoday-go)](https://goreportcard.com/report/github.com/AllRates-Today/allratestoday-go)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![zero dependencies](https://img.shields.io/badge/dependencies-0-brightgreen.svg)](go.mod)
[![Powered by AllRatesToday](https://img.shields.io/badge/Powered%20by-AllRatesToday-orange.svg)](https://allratestoday.com)

## 🚀 Features

- ⚡ **Real-time rates** — latest mid-market rates for any base currency
- 🌍 **160+ currencies** — the full list is available keylessly via `Symbols`
- 📅 **Historical data** — a past date, a date range, or a preset period (`1d`, `7d`, `30d`, `1y`) across the last 365 days (paid plans)
- 💱 **Conversion** — convert an amount at the current rate, or at a past date's rate on a paid plan
- 🏛️ **Official rates** — the published tables of [122 central banks and 4 tax authorities](https://allratestoday.com/central-bank-rates-api/institutions/), no key needed
- 🧩 **Typed responses** — every method decodes into a documented struct with JSON tags
- 🛡️ **Typed errors** — non-2xx responses surface as a `*allratestoday.Error` with status code and body
- ⏱️ **Context-aware** — every method takes a `context.Context` for timeouts and cancellation
- 🔧 **Functional options** — `WithBaseURL` and `WithHTTPClient` for testing and custom transports
- 📦 **Zero dependencies** — standard library only (`net/http` + `encoding/json`)
- 💹 **Mid-market rates** — no retail spread baked in

## 🔑 Get your API key

Create a free key at [allratestoday.com/register](https://allratestoday.com/register), no card. Keys look like `art_live_...`, and the SDK sends yours as a Bearer token in the `Authorization` header. `Symbols`, `OfficialRates` and `OfficialSources` work without a key; every other method returns `allratestoday.ErrMissingAPIKey` if the client has none.

A free key (and a 100-call agent trial key) covers the latest rates and conversion. History and past-date rates (`ConvertOnDate`, `Historical`, `TimeSeries`, `GetHistoricalRates`) need a [paid plan](https://allratestoday.com/pricing); on a free or trial key they return a `*allratestoday.Error` with status 403 and code `historical_requires_paid_plan`. The keyed API sends no CORS headers, so use it from server-side code, not the browser.

## 📦 Installation

```bash
go get github.com/AllRates-Today/allratestoday-go@v1.1.1
```

Requires Go 1.22 or later. No third-party dependencies.

> **Upgrading from v1.0.x:** v1.0.x called `/v1/...` paths that the API does not serve, so every call failed with HTTP 404. v1.1.0 keeps the same method names and response types but calls the real endpoints. `Historical` and `TimeSeries` now require at least one symbol and cover the last 365 days.

## 🏁 Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	allratestoday "github.com/AllRates-Today/allratestoday-go"
)

func main() {
	client := allratestoday.NewClient(os.Getenv("ALLRATES_API_KEY"))
	ctx := context.Background()

	// Latest rates for a base currency
	rates, err := client.Latest(ctx, "USD", []string{"EUR", "GBP"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Base:", rates.Base, "as of", rates.Date)
	fmt.Println("Rates:", rates.Rates)

	// Convert an amount
	result, err := client.Convert(ctx, "USD", "EUR", 100)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("100 USD = %.2f EUR\n", result.Result)
}
```

## 📚 API reference

Construct a client with `allratestoday.NewClient(apiKey, opts...)`. Every method takes a `context.Context` first and returns a typed result plus an `error`.

| Method | Endpoint | Key | Returns |
|--------|----------|-----|---------|
| `Latest(ctx, base, symbols)` | `GET /api/v1/rates` | yes | `*LatestResponse` |
| `GetRates(ctx, source, targets...)` | `GET /api/v1/rates` | yes | `[]Rate` |
| `GetRate(ctx, from, to)` | `GET /api/v1/rates` | yes | `*RateResponse` |
| `Convert(ctx, from, to, amount)` | `GET /api/v1/rates?amount=` | yes | `*ConvertResponse` |
| `ConvertOnDate(ctx, from, to, amount, date)` | `GET /api/historical-rates` | paid plan | `*ConvertResponse` |
| `Historical(ctx, date, base, symbols)` | `GET /api/historical-rates` (one call per symbol) | paid plan | `*HistoricalResponse` |
| `TimeSeries(ctx, startDate, endDate, base, symbols)` | `GET /api/historical-rates` (one call per symbol) | paid plan | `*TimeSeriesResponse` |
| `GetHistoricalRates(ctx, source, target, period)` | `GET /api/historical-rates` | paid plan | `*HistoricalRatesResponse` |
| `Symbols(ctx)` | `GET /api/v1/symbols` | no | `*SymbolsResponse` |
| `OfficialRates(ctx, bank, source, target)` | `GET /api/open/central-bank/{bank}` | no | `*OfficialRatesResponse` |
| `OfficialSources(ctx)` | `GET /api/open/central-banks` | no | `*OfficialSourcesResponse` |

### Latest(ctx, base, symbols)

Most recent mid-market rates for a base currency. Pass `nil` for every available currency. `Date` is the observation time, e.g. `2026-10-16T11:31:01+0000`.

```go
// All currencies
rates, err := client.Latest(ctx, "USD", nil)

// Specific currencies only
rates, err := client.Latest(ctx, "USD", []string{"EUR", "GBP", "JPY"})
fmt.Println(rates.Date, rates.Rates["EUR"])
```

### GetRates(ctx, source, targets...) and GetRate(ctx, from, to)

`GetRates` returns the raw list, one `Rate` (`Rate`, `Source`, `Target`, `Time`) per target. `GetRate` is the single-pair shortcut.

```go
list, err := client.GetRates(ctx, "USD", "EUR", "GBP")
for _, r := range list {
	fmt.Println(r.Source, r.Target, r.Rate, r.Time)
}

rate, err := client.GetRate(ctx, "USD", "EUR")
fmt.Println("USD/EUR:", rate.Rate, "at", rate.Date)
```

### Convert(ctx, from, to, amount) and ConvertOnDate(ctx, from, to, amount, date)

Convert at the current rate, or at the rate on a past date (`YYYY-MM-DD`, last 365 days; the closest earlier point is used when the date has none). `ConvertOnDate` needs a paid plan.

```go
result, err := client.Convert(ctx, "USD", "EUR", 250)
fmt.Println(result.Rate, result.Result)

past, err := client.ConvertOnDate(ctx, "USD", "EUR", 250, "2026-09-15")
fmt.Println(past.Date, past.Rate, past.Result)
```

### Historical(ctx, date, base, symbols)

Paid plan. Rates on a past date (`YYYY-MM-DD`, last 365 days). `symbols` is required, and each symbol costs one request. Dates older than 30 days resolve against the weekly 1-year series, so you get the closest earlier weekly point.

```go
rates, err := client.Historical(ctx, "2026-10-01", "USD", []string{"EUR", "GBP"})
fmt.Println(rates.Date, rates.Rates)
```

### TimeSeries(ctx, startDate, endDate, base, symbols)

Paid plan. Rates across a date range, keyed by date then currency. The start date must be within the last 365 days; `symbols` is required, one request per symbol.

```go
series, err := client.TimeSeries(ctx, "2026-10-01", "2026-10-15", "USD", []string{"EUR", "GBP"})
for date, dayRates := range series.Rates {
	fmt.Println(date, dayRates["EUR"], dayRates["GBP"])
}
```

### GetHistoricalRates(ctx, source, target, period)

Paid plan. A pair's history over a preset period: `"1d"`, `"7d"`, `"30d"` or `"1y"` (daily points; weekly for `"1y"`). `Rates` is a slice of `HistoricalRatePoint` (`Date`, `Rate`, `Timestamp` in Unix milliseconds), sorted oldest first.

```go
history, err := client.GetHistoricalRates(ctx, "USD", "EUR", "30d")
for _, point := range history.Rates {
	fmt.Println(point.Date, point.Rate)
}
```

### Symbols(ctx)

Every supported currency, no key needed. `Symbols` maps each code to a `CurrencySymbol` (`Code`, `Name`, `Symbol`); `Currencies` keeps the API's order.

```go
symbols, err := allratestoday.NewClient("").Symbols(ctx)
fmt.Println(symbols.Count, "currencies")
fmt.Println(symbols.Symbols["EUR"].Name, symbols.Symbols["EUR"].Symbol) // Euro €
```

### OfficialRates(ctx, bank, source, target) and OfficialSources(ctx)

The latest official table published by a central bank or tax authority ([122 central banks and 4 tax authorities](https://allratestoday.com/central-bank-rates-api/institutions/): HMRC, US Treasury, Swiss BAZG, India's CBIC). These are the fixed, citable rates for invoicing, tax, customs and accounting, not live market rates. No key needed. Cite `RateDate`, not today's date.

```go
keyless := allratestoday.NewClient("")

// One pair (cross-computed inside the bank's table when not published directly)
ecb, err := keyless.OfficialRates(ctx, "ecb", "EUR", "USD")
fmt.Printf("ECB %s: 1 EUR = %v USD (%s)\n", ecb.RateDate, ecb.Rate, ecb.RateType)

// Full published table
table, err := keyless.OfficialRates(ctx, "ecb", "", "")
for _, row := range table.Rates {
	fmt.Println(row.Base, row.Quote, row.Type, row.Value)
}

// Every source with its latest publication date
sources, err := keyless.OfficialSources(ctx)
for _, s := range sources.Sources {
	fmt.Println(s.Code, s.Name, s.Latest, s.Stale)
}
```

Dated and historical central-bank tables (`/api/v1/central-bank/{bank}/{date}`) need a paid plan; see [pricing](https://allratestoday.com/pricing).

### Options

Override the default host (`https://allratestoday.com`) or the HTTP client:

```go
client := allratestoday.NewClient("art_live_your_api_key",
	allratestoday.WithBaseURL("https://custom.example.com"),
	allratestoday.WithHTTPClient(&http.Client{Timeout: 5 * time.Second}),
)
```

The default HTTP client uses a 10-second timeout; use `context.WithTimeout` for per-request deadlines.

## 🗺️ Currencies covered

160+ currencies, including 🇺🇸 `USD`, 🇪🇺 `EUR`, 🇬🇧 `GBP` and 🇯🇵 `JPY`. Call `Symbols` for the authoritative, always-current list rather than hard-coding one.

## 🛡️ Error handling

Non-2xx API responses come back as a `*allratestoday.Error` carrying the HTTP status code and response body; a missing key is `allratestoday.ErrMissingAPIKey`; transport, validation and decode failures are wrapped standard errors. Use `errors.As` to branch:

```go
rates, err := client.Latest(ctx, "USD", nil)
if err != nil {
	var apiErr *allratestoday.Error
	if errors.As(err, &apiErr) {
		log.Fatalf("API error %d: %s", apiErr.StatusCode, apiErr.Message)
	}
	log.Fatalf("request failed: %v", err) // network, timeout, or parse failure
}
fmt.Println(rates.Rates)
```

## 💡 Notes

- **Attribution for keyless use.** `OfficialRates`, `OfficialSources` and the open endpoints are free with a visible link where the rates are shown: `<a href="https://allratestoday.com">Exchange rates by AllRatesToday</a>`. With an API key it is appreciated, not required. Terms: <https://allratestoday.com/terms/#attribution>.
- **Concurrency.** A `Client` is safe for concurrent use by multiple goroutines — create one and share it.
- **Cancellation.** Every method takes a `context.Context`, so timeouts and cancellation compose with the rest of your service.
- **Request cost.** `Historical` and `TimeSeries` make one request per symbol, because the history endpoint takes one target per call.
- **Tests.** Run the unit tests with `go test ./...`. They use `httptest` with response bodies captured from the live API and never touch the network; set `ALLRATES_API_KEY` to also run the live smoke test (4 keyed requests; on a free key the two history calls are expected to return 403 `historical_requires_paid_plan`).

## 🤖 Use it from an AI agent

The same rates are available to coding agents and LLM tools without a second integration:

- **Claude Code plugin** (no key): `/plugin marketplace add AllRates-Today/claude-code-plugin` then `/plugin install allratestoday@allratestoday`
- **Hosted MCP endpoint**: `https://allratestoday.com/api/mcp` — Streamable HTTP, keyless, nothing to install
- **Local MCP servers**: `npx -y @allratestoday/mcp-server` (live mid-market rates) · `npx -y @allratestoday/central-bank-mcp` (official central-bank rates)
- **Trial key for agents, no signup**: `POST https://allratestoday.com/api/agent/keys` (send `Content-Type: application/json`) returns a 100-call key valid 7 days for latest rates and latest central-bank tables (history and dated tables are paid-plan only); both MCP servers expose it as `request_trial_key`
- **Daily Reference Rate** (first published 2026-10-12): one fixed USD table per business day, the 16:00 London median of minute samples, never revised — keyless at `https://allratestoday.com/api/open/reference-rate`, `get_reference_rate` in the MCP server
- Machine-readable site guide: [llms.txt](https://allratestoday.com/llms.txt)

## 🔗 Links

- **Website:** [allratestoday.com](https://allratestoday.com)
- **API docs:** [allratestoday.com/docs](https://allratestoday.com/docs)
- **Free API key:** [allratestoday.com/register](https://allratestoday.com/register)
- **Package docs:** [pkg.go.dev/github.com/AllRates-Today/allratestoday-go](https://pkg.go.dev/github.com/AllRates-Today/allratestoday-go)
- **Repository:** [github.com/AllRates-Today/allratestoday-go](https://github.com/AllRates-Today/allratestoday-go)
- **Status:** [allratestoday.com/status](https://allratestoday.com/status)
- **Postman:** [allratestoday.com/postman](https://allratestoday.com/postman/)
- **Support:** [allratestoday.com/contact](https://allratestoday.com/contact)
- **Dashboard:** [allratestoday.com/profile](https://allratestoday.com/profile)

## 📜 License

MIT — see [LICENSE](LICENSE).
