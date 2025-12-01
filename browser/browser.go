package browser

import "github.com/playwright-community/playwright-go"

func LaunchFirefox(options ...playwright.BrowserTypeLaunchOptions) (playwright.Browser, error) {
	pw, err := playwright.Run(
		&playwright.RunOptions{
			Browsers: []string{"firefox"},
		},
	)

	if err != nil {
		return nil, err
	}

	return pw.Firefox.Launch(options...)
}
