package service

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestInvalidateCache(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	s := &RedirectService{rdb: rdb}
	ctx := context.Background()

	// Set a cache entry with host in key
	host := "example.com"
	code := "abc123"
	cacheKey := linkCacheKeyPrefix + host + ":" + code
	mr.Set(cacheKey, `{"url":"https://test.com","is_active":true}`)

	// Invalidate should remove the entry
	err = s.InvalidateCache(ctx, host, code)
	if err != nil {
		t.Fatalf("InvalidateCache failed: %v", err)
	}

	// Verify key is deleted
	if mr.Exists(cacheKey) {
		t.Error("cache key should have been deleted")
	}
}
