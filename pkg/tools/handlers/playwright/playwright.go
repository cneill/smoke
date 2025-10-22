package playwright

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/playwright-community/playwright-go"
)

const (
	ParamURL = "url"
)

type Playwright struct {
	ProjectPath string
}

func New(projectPath, _ string) (tools.Tool, error) {
	// TODO: check for dependencies?
	return &Playwright{ProjectPath: projectPath}, nil
}

func (p *Playwright) Name() string { return tools.NamePlaywright }
func (p *Playwright) Description() string {
	examples := tools.CollectExamples(p.Examples()...)

	return "Take a screenshot with Playwright." + examples
}

func (p *Playwright) Examples() tools.Examples {
	// TODO
	return tools.Examples{}
}

func (p *Playwright) Params() tools.Params {
	return tools.Params{
		{
			Key:         ParamURL,
			Description: "The URL of the page to take a screenshot of",
			Type:        tools.ParamTypeString,
			Required:    true,
		},
	}
}

func (p *Playwright) Run(_ context.Context, args tools.Args) (*tools.Output, error) {
	url := args.GetString(ParamURL)
	if url == nil || *url == "" {
		return nil, fmt.Errorf("%w: must supply URL", tools.ErrArguments)
	}

	screenshotPath, err := fs.GetRelativePath(p.ProjectPath, fmt.Sprintf("screenshot-%s.png", time.Now().Format(time.RFC3339)))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid screenshot path: %w", tools.ErrFileSystem, err)
	}

	// TODO: whitelist/blacklist

	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run playwright: %w", err)
	}

	defer func() {
		if err := pw.Stop(); err != nil {
			slog.Error("failed to stop playwright", "error", err)
		}
	}()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to launch Chromium for playwright: %w", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create a new browser page in playwright: %w", err)
	}

	_, err = page.Goto(*url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to go to URL %q with playwright: %w", *url, err)
	}

	_, err = page.Screenshot(playwright.PageScreenshotOptions{
		Path:     playwright.String(screenshotPath),
		FullPage: playwright.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot with playwright: %w", err)
	}

	output := &tools.Output{
		ImagePath: screenshotPath,
	}

	return output, nil
}
