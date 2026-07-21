package playwright

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cneill/smoke/pkg/fs"
	"github.com/cneill/smoke/pkg/tools"
	"github.com/mxschmitt/playwright-go"
)

const (
	ParamURL      = "url"
	ParamFullPage = "full_page"
)

type Playwright struct {
	ProjectPath string
}

func New(projectPath, _ string) (tools.Tool, error) {
	err := playwright.Install(&playwright.RunOptions{
		OnlyInstallShell: true,
		DriverDirectory:  "/tmp/playwright",
		Browsers:         []string{"chromium"},
		Logger:           slog.Default(),
		WithDeps:         true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set up Playwright dependencies: %w", err)
	}

	return &Playwright{ProjectPath: projectPath}, nil
}

func (p *Playwright) Name() string { return tools.NamePlaywright }
func (p *Playwright) Description() string {
	examples := tools.CollectExamples(p.Examples()...)

	return "Take a screenshot with Playwright." + examples
}

func (p *Playwright) Examples() tools.Examples {
	return tools.Examples{
		{
			Description: "Take a 1280x720 screenshot of a local webpage.",
			Args: tools.Args{
				ParamURL: "http://localhost:8080/",
			},
		},
		{
			Description: "Take a screenshot of the entire Google homepage.",
			Args: tools.Args{
				ParamURL:      "https://google.com/",
				ParamFullPage: true,
			},
		},
	}
}

func (p *Playwright) Params() tools.Params {
	return tools.Params{
		{
			Key:         ParamURL,
			Description: "The URL of the page to take a screenshot of",
			Type:        tools.ParamTypeString,
			Required:    true,
		},
		{
			Key:         ParamFullPage,
			Description: "Should we take a screenshot of the entire page?",
			Type:        tools.ParamTypeBoolean,
			Required:    false,
		},
	}
}

func (p *Playwright) Run(_ context.Context, args tools.Args) (*tools.Output, error) {
	url := args.GetString(ParamURL)
	if url == nil || *url == "" {
		return nil, fmt.Errorf("%w: must supply %q", tools.ErrArguments, ParamURL)
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

	defer func() {
		if err := browser.Close(); err != nil {
			slog.Error("failed to close browser", "error", err)
		}
	}()

	screenshotPath, err := p.takeScreenshot(browser, *url, args.GetBool(ParamFullPage))
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot with playwright: %w", err)
	}

	output := &tools.Output{
		ImagePath: screenshotPath,
	}

	return output, nil
}

func (p *Playwright) takeScreenshot(browser playwright.Browser, url string, full *bool) (string, error) {
	page, err := browser.NewPage()
	if err != nil {
		return "", fmt.Errorf("failed to create a new browser page in playwright: %w", err)
	}

	gotoOpts := playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}

	if _, err = page.Goto(url, gotoOpts); err != nil {
		return "", fmt.Errorf("failed to go to URL %q with playwright: %w", url, err)
	}

	screenshotPath, _ := fs.GetRelativePath(p.ProjectPath, fmt.Sprintf("screenshot-%s.png", time.Now().Format(time.RFC3339)))
	screenshotOpts := playwright.PageScreenshotOptions{
		Path: playwright.String(screenshotPath),
	}

	if full != nil && *full {
		screenshotOpts.FullPage = full
	} else {
		screenshotOpts.Clip = &playwright.Rect{
			Width:  1280,
			Height: 720,
		}
	}

	if _, err = page.Screenshot(screenshotOpts); err != nil {
		return "", fmt.Errorf("failed to take screenshot with playwright: %w", err)
	}

	return screenshotPath, nil
}
