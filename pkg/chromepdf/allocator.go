package chromepdf

import (
	"context"
	"os"

	"github.com/chromedp/chromedp"
)

func isProduction() bool {
	// If the production binary exists, we are running inside our Docker image
	_, err := os.Stat("/usr/bin/google-chrome-stable")
	return err == nil || os.Getenv("CHROMIUM_PATH") != ""
}

func chromiumPath() string {
	if p := os.Getenv("CHROMIUM_PATH"); p != "" {
		return p
	}
	if isProduction() {
		return "/usr/bin/google-chrome-stable"
	}
	return "" // Auto-discover locally
}

func newAllocator(ctx context.Context) (context.Context, context.CancelFunc) {
	// Start with standard default options
	opts := chromedp.DefaultExecAllocatorOptions[:]

	if path := chromiumPath(); path != "" {
		opts = append(opts, chromedp.ExecPath(path))
	}

	if isProduction() {
		// Production Container Flags: Heavy isolation required for rootless Linux containers
		opts = append(opts,
			chromedp.Flag("headless", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.Flag("disable-setuid-sandbox", true),
			chromedp.Flag("no-zygote", true),
			chromedp.Flag("single-process", true),
			chromedp.Flag("disable-extensions", true),
			chromedp.Flag("mute-audio", true),
			chromedp.Flag("no-crashpad", true),
			chromedp.Flag("disable-crash-reporter", true),
			chromedp.Flag("crash-dumps-dir", "/tmp"),
			// chromedp.Flag("disable-background-networking", true),
			chromedp.Flag("disable-default-apps", true),
			chromedp.Flag("disable-features", "CrashpadHandlerOnBrowser"),
			chromedp.UserDataDir("/tmp/chromedp-profile"),
		)
	} else {
		// Local Development Flags: Light, safe flags that won't trigger local OS security crashes
		opts = append(opts,
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("mute-audio", true),
		)
	}

	return chromedp.NewExecAllocator(ctx, opts...)
}
