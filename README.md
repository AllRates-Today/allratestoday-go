# AllRatesToday Go SDK — allratestoday-go

The official Go client for the [AllRatesToday](https://allratestoday.com) currency API. A small, zero-dependency wrapper over `net/http` for Go services and CLI tools that need real-time, historical or time-series exchange rates without hand-rolling request and response types.

[![Go Reference](https://pkg.go.dev/badge/github.com/AllRates-Today/allratestoday-go.svg)](https://pkg.go.dev/github.com/AllRates-Today/allratestoday-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/AllRates-Today/allratestoday-go)](https://goreportcard.com/report/github.com/AllRates-Today/allratestoday-go)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![zero dependencies](https://img.shields.io/badge/dependencies-0-brightgreen.svg)](go.mod)
[![Powered by AllRatesToday](https://img.shields.io/badge/Powered%20by-AllRatesToday-orange.svg)](https://allratestoday.com)

## 🚀 Features

- ⚡ **Real-time rates** — latest mid-market rates for any base currency
- 🌍 **160+ currencies** — the full list is available from the API via `Symbols`
- 📅 **Historical data** — rates for a single past date, an arbitrary date range, or a preset period (`1d`, `7d`, `30d`, `1y`)
- 💱 **Conversion** — convert an amount between two currencies in one call
- 🧩 **Typed responses** — every endpoint decodes into a documented struct with JSON tags
- 🛡️ **Typed errors** — non-2xx responses surface as a `*allratestoday.Error` with status code and body
- ⏱️ **Context-aware** — every method takes a `context.Context` for timeouts and cancellation
- 🔧 **Functional options** — `WithBaseURL` and `WithHTTPClient` for testing and custom transports
- 📦 **Zero dependencies** — standard library only (`net/http` + `encoding/json`)
- 💹 **Mid-market rates** — no retail spread baked in

## 🔑 Get your API key

Create a free key at [allratestoday.com/register](https://allratestoday.com/register). Keys look like `art_live_...`, and the SDK sends yours as a Bearer token in the `Authorization` header on every request.

## 📦 Installation

```bash
go get github.com/AllRates-Today/allratestoday-go
```

Requires Go 1.22 or later. No third-party dependencies.

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
	fmt.Println("Base:", rates.Base)
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

Construct a client with `allratestoday.NewClient(apiKey, opts...)`. Every method takes a `context.Context` first and returns a typed response pointer plus an `error`.

| Method | Endpoint | Returns |
|--------|----------|---------|
| `Latest(ctx, base, symbols)` | `GET /v1/latest` | `*LatestResponse` |
| `Convert(ctx, from, to, amount)` | `GET /v1/convert` | `*ConvertResponse` |
| `Historical(ctx, date, base, symbols)` | `GET /v1/historical` | `*HistoricalResponse` |
| `TimeSeries(ctx, startDate, endDate, base, symbols)` | `GET /v1/timeseries` | `*TimeSeriesResponse` |
| `Symbols(ctx)` | `GET /v1/symbols` | `*SymbolsResponse` |
| `GetRate(ctx, from, to)` | `GET /v1/rate` | `*RateResponse` |
| `GetHistoricalRates(ctx, source, target, period)` | `GET /v1/historical-rates` | `*HistoricalRatesResponse` |

`symbols` parameters take a `[]string`; pass `nil` for every available currency.

### Latest(ctx, base, symbols)

Most recent rates for a base currency.

```go
// All currencies
rates, err := client.Latest(ctx, "USD", nil)

// Specific currencies only
rates, err := client.Latest(ctx, "USD", []string{"EUR", "GBP", "JPY"})
fmt.Println(rates.Date, rates.Rates)
```

### Convert(ctx, from, to, amount)

Convert an amount between two currencies.

```go
result, err := client.Convert(ctx, "USD", "EUR", 250)
fmt.Println(result.Rate, result.Result)
```

### Historical(ctx, date, base, symbols)

Rates as of a specific historical date (`YYYY-MM-DD`).

```go
rates, err := client.Historical(ctx, "2025-01-15", "USD", []string{"EUR", "GBP"})
fmt.Println(rates.Date, rates.Rates)
```

### TimeSeries(ctx, startDate, endDate, base, symbols)

Rates across a date range, keyed by date.

```go
series, err := client.TimeSeries(ctx, "2025-01-01", "2025-01-31", "USD", []string{"EUR", "GBP"})
for date, dayRates := range series.Rates {
	fmt.Println(date, dayRates)
}
```

### Symbols(ctx)

Every supported currency code with its details.

```go
symbols, err := client.Symbols(ctx)
for code, info := range symbols.Symbols {
	fmt.Println(code, info.Name)
}
```

### GetRate(ctx, from, to)

A single currency pair.

```go
rate, err := client.GetRate(ctx, "USD", "EUR")
fmt.Println("USD/EUR:", rate.Rate)
```

### GetHistoricalRates(ctx, source, target, period)

A pair's history over a preset period — `"1d"`, `"7d"`, `"30d"` or `"1y"`. Returns a slice of `HistoricalRatePoint` (`Date` / `Rate` pairs).

```go
history, err := client.GetHistoricalRates(ctx, "USD", "EUR", "30d")
for _, point := range history.Rates {
	fmt.Println(point.Date, point.Rate)
}
```

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

Non-2xx API responses come back as a `*allratestoday.Error` carrying the HTTP status code and response body; transport and decode failures are wrapped standard errors. Use `errors.As` to branch:

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

- **Concurrency.** A `Client` is safe for concurrent use by multiple goroutines — create one and share it.
- **Cancellation.** Every method takes a `context.Context`, so timeouts and cancellation compose with the rest of your service.
- **Tests.** Run the unit tests with `go test ./...`. They use `httptest` and never touch the network; set `ALLRATES_API_KEY` to also run the live smoke test.

## 🔗 Links

- **Website:** [allratestoday.com](https://allratestoday.com)
- **API docs:** [allratestoday.com/docs](https://allratestoday.com/docs)
- **Free API key:** [allratestoday.com/register](https://allratestoday.com/register)
- **Package docs:** [pkg.go.dev/github.com/AllRates-Today/allratestoday-go](https://pkg.go.dev/github.com/AllRates-Today/allratestoday-go)
- **Repository:** [github.com/AllRates-Today/allratestoday-go](https://github.com/AllRates-Today/allratestoday-go)
- **Status:** [allratestoday.com/status](https://allratestoday.com/status)
- **Support:** [allratestoday.com/contact](https://allratestoday.com/contact)

## 📜 License

MIT — see [LICENSE](LICENSE).
