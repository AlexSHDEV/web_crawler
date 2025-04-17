package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"main/internal/db"
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
	Mode       bool              `json:"spider_mode"`
	ToDownload []string          `json:"toDownload"`
	DBConfig   db.DatabaseConfig `json:"dbconfig"`
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

func hashMD5(content string) string {
	hash := md5.Sum([]byte(content))
	return hex.EncodeToString(hash[:])
}

func main() {

	// ============================<INIT BLOCK>=================================

	ctx := context.Background()
	cache := downloader.NewDNSCache("localhost:6379", 1*time.Hour) // ===================================================== !
	resolver := downloader.NewDNSResolver(dnsServers, *cache)
	settings := &settings{}
	settings.SetSettings(queuePath)

	fmt.Println(settings)

	storage, err := db.NewPostgresStorage(settings.DBConfig)
	if err != nil {
		fmt.Println("Error create connection to database: ", err)
		//log.Fatalf("Failed to initialize storage: %v", err)
		return
	}
	defer storage.Close()

	if err := storage.Init(); err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}

	// =========================================================================

	for _, url := range settings.ToDownload {

		fmt.Println("Started with URL: ", url)

		htmlPage, err := downloader.FetchDynamicHTML(ctx, url, resolver)
		if err != nil {
			fmt.Println("Error fetching HTML: ", err)
			return
		}
		host, err := downloader.GetHost(url)
		if err != nil {
			fmt.Printf("Getting host from url falied: %v\n", err)
			return
		}

		content := &db.CrawledContent{
			DOMAIN:      host,
			URL:         url,
			TextContent: "",
			Title:       "",
			Metadata:    nil,
			ContentHash: hashMD5(htmlPage),
			CrawledAt:   time.Now(),
		}

		exists, err := storage.Exists(context.Background(), content.ContentHash)
		if err != nil {
			log.Printf("Error checking content: %v", err)
		}
		if exists {
			log.Println("Content already exists, skipping")
			continue
		}

		if err := storage.Save(context.Background(), content); err != nil {
			log.Printf("Failed to save content: %v", err)
		} else {
			log.Println("Content saved successfully")
		}

		links := downloader.ExtractLinks(htmlPage, url)
		for _, link := range links {
			fmt.Println(link)
		}
	}
}
