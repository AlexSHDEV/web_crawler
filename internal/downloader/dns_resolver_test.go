package downloader

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDNSCache(t *testing.T) {
	// Setup in-memory Redis for testing
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	ctx := context.Background()
	ttl := 10 * time.Minute
	cache := NewDNSCache(mr.Addr(), ttl)

	t.Run("Get non-existent key", func(t *testing.T) {
		ips, err := cache.Get(ctx, "nonexistent.com")
		assert.NoError(t, err)
		assert.Nil(t, ips)
	})

	t.Run("Set and Get", func(t *testing.T) {
		testIPs := []net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("2606:4700:4700::1111")}

		err := cache.Set(ctx, "example.com", testIPs)
		assert.NoError(t, err)

		// Verify TTL was set
		ttlRemaining := mr.TTL("dns:example.com")
		assert.True(t, ttlRemaining > 0 && ttlRemaining <= ttl)

		// Get the value back
		ips, err := cache.Get(ctx, "example.com")
		assert.NoError(t, err)
		assert.Equal(t, testIPs, ips)
	})

	t.Run("Get with invalid data", func(t *testing.T) {
		mr.Set("dns:bad.com", "invalid json")
		ips, err := cache.Get(ctx, "bad.com")
		assert.Error(t, err)
		assert.Nil(t, ips)
	})

	t.Run("Redis connection error", func(t *testing.T) {
		// Close the Redis server to simulate connection error
		mr.Close()
		ips, err := cache.Get(ctx, "example.com")
		assert.Error(t, err)
		assert.Nil(t, ips)

		// Reopen for other tests
		mr.Start()
	})
}

func TestDNSResolver(t *testing.T) {
	// Setup in-memory Redis for testing
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	ctx := context.Background()
	ttl := 10 * time.Minute
	cache := NewDNSCache(mr.Addr(), ttl)

	// Mock DNS servers (we'll use system resolver in tests)
	servers := []string{"8.8.8.8", "1.1.1.1"}
	resolver := NewDNSResolver(servers, *cache)

	t.Run("ResolveWithPreference - IPv4", func(t *testing.T) {
		host := "google.com" // Assuming this has both IPv4 and IPv6

		ip, err := resolver.ResolveWithPreference(ctx, host, false)
		assert.NoError(t, err)
		assert.NotNil(t, ip)
		assert.NotNil(t, ip.To4(), "should return IPv4 address")
	})

	t.Run("ResolveWithPreference - IPv6", func(t *testing.T) {
		host := "google.com" // Assuming this has IPv6

		ip, err := resolver.ResolveWithPreference(ctx, host, true)
		assert.NoError(t, err)
		assert.NotNil(t, ip)
		if ip.To4() == nil {
			// Only check for IPv6 if the host actually has IPv6
			assert.Nil(t, ip.To4(), "should return IPv6 address")
		}
	})

	t.Run("ResolveWithPreference - fallback", func(t *testing.T) {
		// This test assumes example.com has at least one IP address
		host := "example.com"

		ip, err := resolver.ResolveWithPreference(ctx, host, true)
		assert.NoError(t, err)
		assert.NotNil(t, ip)
	})

	t.Run("Resolve invalid host", func(t *testing.T) {
		_, err := resolver.Resolve(ctx, "invalid.hostname.that.does.not.exist")
		assert.Error(t, err)
	})
}

func TestIpVersion(t *testing.T) {
	tests := []struct {
		ip      string
		version string
	}{
		{"1.1.1.1", "IPv4"},
		{"8.8.8.8", "IPv4"},
		{"2606:4700:4700::1111", "IPv6"},
		{"::1", "IPv6"},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			assert.Equal(t, tt.version, IpVersion(ip))
		})
	}
}
