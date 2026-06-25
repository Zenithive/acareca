package chromepdf

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

func Generate(ctx context.Context, html string) ([]byte, error) {
	allocCtx, cancelAlloc := newAllocator(ctx)
	defer cancelAlloc()

	chromeCtx, cancelChrome := chromedp.NewContext(allocCtx)
	defer cancelChrome()

	dataURI := "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(html))

	var pdfBuf []byte
	if err := chromedp.Run(chromeCtx,
		chromedp.Navigate(dataURI),
		chromedp.ActionFunc(func(ctx context.Context) error {
			buf, _, err := page.PrintToPDF().
				WithPrintBackground(true).
				WithPaperWidth(8.27).   // A4 width in inches
				WithPaperHeight(11.69). // A4 height in inches
				WithMarginTop(0.4).
				WithMarginBottom(0.4).
				WithMarginLeft(0.4).
				WithMarginRight(0.4).
				Do(ctx)
			if err != nil {
				return fmt.Errorf("PrintToPDF failed: %w", err)
			}
			pdfBuf = buf
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("chromedp run failed: %w", err)
	}

	return pdfBuf, nil
}
