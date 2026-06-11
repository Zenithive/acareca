package chromepdf

import (
	"context"
	"os"

	"github.com/chromedp/chromedp"
)

func chromiumPath() string {
	if p := os.Getenv("CHROMIUM_PATH"); p != "" {
		return p
	}
	return "/usr/bin/chromium"
}

func newAllocator(ctx context.Context) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromiumPath()),
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
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-features", "CrashpadHandlerOnBrowser"),
		chromedp.UserDataDir("/tmp/chromedp-profile"),
	)
	return chromedp.NewExecAllocator(ctx, opts...)
}
