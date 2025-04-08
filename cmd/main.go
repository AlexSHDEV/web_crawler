package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"golang.org/x/net/html"
)

var queuePath string = "././queue.json"

type settings struct {
	Mode       bool     `json:"spider_mode"`
	ToDownload []string `json:"toDownload"`
}

func (s *settings) SetSettings(fileName string) {
	jsonFile, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Ошибка открытия файла:", err)
		return
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		fmt.Println("Ошибка чтения файла: ", err)
		return
	}

	err = json.Unmarshal(byteValue, &s)
	if err != nil {
		fmt.Println("Ошибка декодирования JSON: ", err)
	}
}

func fetchHTML(url string) (*html.Node, error) {
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

func extractLinks(n *html.Node, baseURL string) []string {
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

func main() {
	settings := &settings{}
	settings.SetSettings(queuePath)

	for _, url := range settings.ToDownload {
		fmt.Println("Start with URL: ", url)

		node, err := fetchHTML(url)
		if err != nil {
			fmt.Println("Error fetching HTML: ", err)
			return
		}

		links := extractLinks(node, url)
		for _, link := range links {
			fmt.Println(link)
		}
	}
}
