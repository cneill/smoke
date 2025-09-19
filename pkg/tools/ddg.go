package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/cneill/hc/v2"
)

const (
	DDGQuery = "query"

	ddgLinkPrefix = "//duckduckgo.com/l/?uddg="
)

type DDGTool struct {
	ProjectPath string
	hc          *hc.HC
}

func NewDDGTool(projectPath, _ string) Tool {
	headers := http.Header{}
	headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	headers.Set("Accept-Language", "en-US,en;q=0.5")
	headers.Set("Referer", "https://duckduckgo.com")
	headers.Set("User-Agent", "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:141.0) Gecko/20100101 Firefox/141.0")

	hc, err := hc.New(hc.DefaultClient(),
		hc.ClientBaseURL("https://html.duckduckgo.com/html"),
		hc.GlobalHeaders(headers),
		hc.RateTickerDuration(time.Second*2),
	)
	if err != nil {
		panic(fmt.Errorf("failed to set up HC: %w", err))
	}

	return &DDGTool{
		ProjectPath: projectPath,
		hc:          hc,
	}
}

func (d *DDGTool) Name() string { return ToolDDG }
func (d *DDGTool) Description() string {
	examples := CollectExamples(d.Examples()...)

	return fmt.Sprintf(
		"Retrieve one page of results from the DuckDuckGo search engine based on the query supplied in %q.%s",
		DDGQuery, examples)
}

func (d *DDGTool) Examples() Examples {
	return Examples{
		{
			Description: `Search for the string "HTML"`,
			Args: Args{
				DDGQuery: "HTML",
			},
		},
	}
}

func (d *DDGTool) Params() Params {
	return Params{
		{
			Key:         DDGQuery,
			Description: "The query to search DuckDuckGo for",
			Type:        ParamTypeString,
			Required:    true,
		},
	}
}

func (d *DDGTool) Run(ctx context.Context, args Args) (string, error) {
	query := args.GetString(DDGQuery)
	if query == nil {
		return "", fmt.Errorf("no query supplied")
	}

	resp, err := d.hc.Do(
		hc.Context(ctx),
		hc.Query("q", *query),
	)
	if err != nil {
		return "", fmt.Errorf("error with request: %w", err)
	}

	defer resp.Body.Close()

	results, err := d.getResults(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to get DuckDuckGo results: %w", err)
	}

	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("failed to convert DuckDuckGo results to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

type ddgResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

func (d *DDGTool) getResults(body io.Reader) ([]ddgResult, error) {
	results := []ddgResult{}

	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document from response body: %w", err)
	}

	for resultIdx, rawResult := range doc.Find(".results .result .result__body").EachIter() {
		snippet := rawResult.Find(".result__snippet").First().Text()

		aTag := rawResult.Find(".result__title .result__a").First()
		title := aTag.Text()

		href, ok := aTag.Attr("href")
		if !ok {
			slog.Warn("got result item without an href attribute...", "idx", resultIdx, "title", title)
			continue
		}

		// Links come back with some funky redirect formatting that looks like e.g. this when JSON-escaped:
		// //duckduckgo.com/l/?uddg=https%3A%2F%2Fwww.thetexastasty.com%2Faround%2Dtown%2Faustin%2Fbest%2Dsushi%2Din%2D
		// austin%2F\\u0026rut=abc6008f580ac8d5fd52d7d87628fe411ab71c611cd05e3fd7d508165656f956
		if cleaned, found := strings.CutPrefix(href, ddgLinkPrefix); found {
			cleaned = strings.Split(cleaned, "&")[0]

			unescaped, err := url.QueryUnescape(cleaned)
			if err != nil {
				slog.Error("failed to unescape DDG result link", "idx", resultIdx, "raw", href, "cleaned", cleaned)
				continue
			}

			href = unescaped
		}

		if _, err := url.Parse(href); err != nil {
			slog.Warn("got invalid URL for result", "idx", resultIdx, "href", href)
			continue
		}

		result := ddgResult{
			Title:   title,
			URL:     href,
			Snippet: snippet,
		}

		results = append(results, result)
	}

	return results, nil
}
