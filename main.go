package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"main/internal/downloader"
	"os"
	"time"
)

// Тестовые домены
var domains = []string{ // ===================================================== !
	"google.com",
	"youtube.com",
	"github.com",
	"reddit.com",
	"stackoverflow.com",
	"msu.ru",
}

// Список публичных DNS-серверов
var dnsServers = []string{ // ===================================================== !
	"1.1.1.1",        // Cloudflare
	"8.8.8.8",        // Google
	"9.9.9.9",        // Quad9
	"208.67.222.222", // OpenDNS
}

var queuePath string = "./queue.json"

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

func main() {
	ctx := context.Background()
	cache := downloader.NewDNSCache("localhost:6379", 1*time.Hour) // ===================================================== !
	resolver := downloader.NewDNSResolver(dnsServers, *cache)
	settings := &settings{}
	settings.SetSettings(queuePath)

	for _, url := range settings.ToDownload {

		fmt.Println("Started with URL: ", url)

		node, err := downloader.FetchDynamicHTML(ctx, url, resolver)

		if err != nil {
			fmt.Println("Error fetching HTML: ", err)
			return
		}

		links := downloader.ExtractLinks(node, url)
		for _, link := range links {
			fmt.Println(link)
		}
	}
}
