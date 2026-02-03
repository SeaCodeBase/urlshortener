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

func TestInvalidateCache_CustomDomain(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	s := &RedirectService{rdb: rdb}
	ctx := context.Background()

	// Simulate cached link for custom domain
	host := "short.example.com"
	code := "xyz789"
	cacheKey := linkCacheKeyPrefix + host + ":" + code
	mr.Set(cacheKey, `{"url":"https://original.com","is_active":true,"link_id":123}`)

	// Verify it exists
	if !mr.Exists(cacheKey) {
		t.Fatal("test setup: cache key should exist")
	}

	// Invalidate
	err = s.InvalidateCache(ctx, host, code)
	if err != nil {
		t.Fatalf("InvalidateCache failed: %v", err)
	}

	// Verify deleted
	if mr.Exists(cacheKey) {
		t.Error("cache key should have been deleted after invalidation")
	}
}

func TestInvalidateCache_NonExistentKey(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	s := &RedirectService{rdb: rdb}
	ctx := context.Background()

	// Invalidating non-existent key should not error
	err = s.InvalidateCache(ctx, "nonexistent.com", "missing")
	if err != nil {
		t.Errorf("InvalidateCache should not error for non-existent key: %v", err)
	}
}
