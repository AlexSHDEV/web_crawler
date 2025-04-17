package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
)

type DNSCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewDNSCache(addr string, ttl time.Duration) *DNSCache {
	return &DNSCache{
		client: redis.NewClient(&redis.Options{ // ===================================================== !
			Addr:     addr,
			Password: "", // no password set
			DB:       0,  // use default DB
		}),
		ttl: ttl,
	}
}

func (dc *DNSCache) Get(ctx context.Context, host string) ([]net.IP, error) {
	val, err := dc.client.Get(ctx, "dns:"+host).Result()
	if err == redis.Nil {
		return nil, nil // Ключ не найден - это не ошибка
	} else if err != nil {
		return nil, err
	}

	var ips []net.IP
	if err := json.Unmarshal([]byte(val), &ips); err != nil {
		return nil, err
	}
	return ips, nil
}

func (dc *DNSCache) Set(ctx context.Context, host string, ips []net.IP) error {
	data, err := json.Marshal(ips)
	if err != nil {
		return err
	}
	return dc.client.Set(ctx, "dns:"+host, data, dc.ttl).Err()
}

// DNSResolver - кастомный DNS-резолвер с кешированием
type DNSResolver struct {
	cache   *DNSCache
	servers []string
}

func NewDNSResolver(servers []string, dnscache DNSCache) *DNSResolver {
	return &DNSResolver{
		cache:   &dnscache,
		servers: servers,
	}
}

func (r *DNSResolver) Resolve(ctx context.Context, host string) ([]net.IP, error) {
	// Пытаемся получить из кеша
	if cached, err := r.cache.Get(ctx, host); err != nil {
		return nil, fmt.Errorf("cache error: %v", err)
	} else if cached != nil {
		fmt.Printf("[CACHE HIT] %s\n", host)
		return cached, nil
	}

	fmt.Printf("[CACHE MISS] %s\n", host)

	// Выполняем DNS-запрос
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 10 * time.Second}
			server := r.servers[rand.Intn(len(r.servers))]
			return d.DialContext(ctx, "udp", server+":53")
		},
	}

	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}

	var result []net.IP
	for _, ip := range ips {
		result = append(result, ip.IP)
	}

	// Сохраняем в кеш
	if err := r.cache.Set(ctx, host, result); err != nil {
		fmt.Printf("Warning: failed to cache DNS result: %v\n", err)
	}

	return result, nil
}

// ResolveWithPreference разрешает домен с предпочтением IPv4/IPv6
func (r *DNSResolver) ResolveWithPreference(ctx context.Context, host string, preferIPv6 bool) (net.IP, error) {
	ips, err := r.Resolve(ctx, host)
	if err != nil {
		return nil, err
	}

	// Разделение IPv4 и IPv6 адресов
	var ipv4, ipv6 []net.IP
	for _, ip := range ips {
		if ip.To4() != nil {
			ipv4 = append(ipv4, ip)
		} else {
			ipv6 = append(ipv6, ip)
		}
	}

	// Выбор адреса согласно предпочтению
	if preferIPv6 && len(ipv6) > 0 {
		return ipv6[0], nil
	}
	if len(ipv4) > 0 {
		return ipv4[0], nil
	}
	if len(ipv6) > 0 {
		return ipv6[0], nil
	}

	return nil, fmt.Errorf("no IP addresses found")
}

// ipVersion возвращает версию IP-адреса
func IpVersion(ip net.IP) string {
	if ip.To4() != nil {
		return "IPv4"
	}
	return "IPv6"
}
