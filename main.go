package main

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"main/internal/db"
	"main/internal/downloader"
	"os"
	"strings"
	"sync"
	"time"
)

var settingsPath string = "./settings.json"

type settings struct {
	Mode        string            `json:"mode"`
	MainHost    string            `json:"main_domain"`
	DnsServers  []string          `json:"dns_servers"`
	ToDownload  string            `json:"toDownload"`
	DBConfig    db.DatabaseConfig `json:"dbconfig"`
	RedisConfig struct {
		Host       string `json:"host"`
		Expiration int    `json:"expiration"`
	} `json:"redisconfig"`
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

func hashSHA256(content string) string {
	hasher := sha256.New()
	hasher.Write([]byte(content))
	return hex.EncodeToString(hasher.Sum(nil))
}

type Worker struct {
	id       int
	resolver *downloader.DNSResolver
	storage  *db.PostgresStorage
	wg       *sync.WaitGroup
	timeout  time.Duration
	pool     downloader.URLsPool
	host     string
}

func (w *Worker) Start(ctx context.Context, in chan string) {
	defer w.wg.Done()

	for {
		select {
		case url := <-in:
			htmlPage, status, err := downloader.FetchDynamicHTML(ctx, url, w.resolver)

			if err != nil {
				fmt.Println("Error fetching HTML: ", err)
				continue
			}
			host, err := downloader.GetHost(url)
			if err != nil {
				fmt.Printf("Getting host from url falied: %v\n", err)
				continue
			}

			content := &db.CrawledContent{
				DOMAIN:      host,
				URL:         url,
				TextContent: "empty",
				Title:       "empty",
				Status:      status,
				Metadata:    nil,
				ContentHash: hashMD5(htmlPage[int(float64(len(htmlPage))*0.8):]),
				CrawledAt:   time.Now(),
			}

			// =================<>==================

			exists, err := w.storage.ExistsByURL(context.Background(), content.URL)

			if err != nil {
				log.Printf("Error checking content: %v", err)
			}
			if exists {
				log.Println("Content already exists, skipping ", content.URL, " by worker ", w.id)
				continue
			}

			if err := w.storage.Save(context.Background(), content); err != nil {
				log.Printf("Failed to save content: %v", err)
			} else {
				log.Println("Content saved successfully ", content.URL, " by worker ", w.id)
			}

			if host == w.host || strings.Contains(host, w.host) {
				links := downloader.ExtractLinks(htmlPage, "http://"+host)

				for _, link := range links {
					if !w.pool.Exist(link) {
						w.pool.Add(link)
						in <- link
					}
				}
			}
		case <-time.After(w.timeout * time.Second):
			fmt.Println("STOP WORKER ", w.id)
			return
		}
	}
}

type Crawler struct {
	resolver *downloader.DNSResolver
	storage  *db.PostgresStorage
}

func BuildCrawler(settings *settings) (*Crawler, error) {
	cache := downloader.NewDNSCache(settings.RedisConfig.Host, time.Duration(settings.RedisConfig.Expiration)*time.Hour)
	storage, err := db.NewPostgresStorage(settings.DBConfig)
	if err != nil {
		fmt.Println("Error create connection to database: ", err)
		//log.Fatalf("Failed to initialize storage: %v", err)
		return nil, err
	}

	if err := storage.Init(); err != nil {
		log.Fatalf("Failed to init database: %v", err)
		return nil, err
	}

	return &Crawler{
		resolver: downloader.NewDNSResolver(settings.DnsServers, *cache),
		storage:  storage,
	}, nil
}

func (c *Crawler) Run(maindomain string, starturl string, numWorkers int) {
	defer c.storage.Close()
	var m sync.RWMutex
	var wg sync.WaitGroup
	ctx := context.Background()
	pool := downloader.CreatePool(&m)

	urlChan := make(chan string, 100000)

	defer close(urlChan)

	urlChan <- starturl

	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		worker := Worker{
			id:       i,
			resolver: c.resolver,
			storage:  c.storage,
			wg:       &wg,
			timeout:  10,
			pool:     *pool,
			host:     maindomain,
		}
		go worker.Start(ctx, urlChan)
	}

	wg.Wait()
}

func (c *Crawler) ShowStat(maindomain string) {
	defer c.storage.Close()
	var out = make([]db.StatContent, 0, 100)
	err := c.storage.GetAll(&out)
	if err != nil {
		return
	}

	fmt.Println("Общее количество ссылок: ", len(out))

	UnderDomain := 0
	for _, row := range out {
		if strings.Contains(row.Domain, maindomain) {
			UnderDomain += 1
		}
	}
	fmt.Println("Количество внутренних ссылок главного домена: ", UnderDomain)

	NotWorked := 0
	for _, row := range out {
		if row.Status != 200 {
			NotWorked += 1
		}
	}
	fmt.Println("Количество неработающих страниц: ", NotWorked)

	InterDomain := 0
	for _, row := range out {
		if strings.Contains(row.Domain, maindomain) && len(row.Domain) > len(maindomain) {
			InterDomain += 1
		}
	}
	fmt.Println("Колличество внутренних поддоменов: ", InterDomain)

	OuterDomain := 0
	UniqOuterDomain := make(map[string]bool, 0)
	for _, row := range out {
		if !strings.Contains(row.Domain, maindomain) {
			OuterDomain += 1
			UniqOuterDomain[row.Domain] = true
		}
	}
	fmt.Println("Колличество ссылок на внешние ресурсы : ", OuterDomain)
	fmt.Println("Количество уникальных внешних ссылок: ", len(UniqOuterDomain))

	Files := 0
	for _, row := range out {
		if strings.Contains(row.Url, ".doc") || strings.Contains(row.Url, ".docx") || strings.Contains(row.Url, ".pdf") {
			Files += 1
		}
	}
	fmt.Println("Количество уникальных ссылок на файлы doc/docx/pdf: ", Files)
}

func main() {

	settings := &settings{}
	settings.SetSettings(settingsPath)
	C, err := BuildCrawler(settings)
	if err != nil {
		log.Fatalf("Failed to Build Crawler: %v", err)
	}

	switch settings.Mode {
	case "spider":
		C.Run(settings.MainHost, settings.ToDownload, 5)
	case "stat":
		C.ShowStat(settings.MainHost)
	}

}
