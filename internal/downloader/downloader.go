package downloader

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"golang.org/x/net/html"
)

func FetchStaticHTML(url string) (*html.Node, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error: status code %d", resp.StatusCode)
	}

	return html.Parse(resp.Body)
}

func GetHost(u string) (string, error) {
	URL, err := url.Parse(u)
	if err != nil {
		return "", fmt.Errorf("Error parsing URL: %d", err)
	}
	return URL.Hostname(), nil
}

func FetchDynamicHTML(ctx context.Context, ur string, resolver *DNSResolver) (string, error) {

	var htmlPage string

	host, err := GetHost(ur)
	if err != nil {
		fmt.Printf("Getting host from url falied: %v\n", err)
		return "", err
	}

	// 2. Разрешаем DNS
	ips, err := resolver.ResolveWithPreference(ctx, host, false)
	if err != nil {
		fmt.Printf("DNS resolution failed: %v\n", err)
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Настраиваем ChromeDP с кастомным DNS
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("host-resolver-rules", fmt.Sprintf("MAP %s %s", host, ips)),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	taskCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	err = chromedp.Run(taskCtx,
		chromedp.Navigate(ur),
		chromedp.WaitReady("body"),
		chromedp.Sleep(2*time.Second),
		chromedp.OuterHTML("html", &htmlPage),
	)
	if err != nil {
		return "", fmt.Errorf("Error running chronedp: %d", err)
	}

	return htmlPage, nil
}

func ExtractLinks(htmlPage string, baseURL string) []string {
	n, err := html.Parse(strings.NewReader(htmlPage))
	if err != nil {
		fmt.Printf("Error parsing html document: %v\n", err)
		return nil
	}

	var links []string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					link := a.Val
					if !strings.HasPrefix(link, "http") {
						link = baseURL + link
					}
					links = append(links, link)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return links
}
