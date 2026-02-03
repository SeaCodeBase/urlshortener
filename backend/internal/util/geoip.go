package util

import (
	"context"
	"net"
	"sync"

	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/oschwald/geoip2-golang"
	"go.uber.org/zap"
)

type GeoIPResult struct {
	Country string
	City    string
}

type GeoIPLookup struct {
	db *geoip2.Reader
	mu sync.RWMutex
}

var geoIPInstance *GeoIPLookup
var geoIPOnce sync.Once

func InitGeoIP(ctx context.Context, dbPath string) error {
	if dbPath == "" {
		return nil // GeoIP disabled
	}

	var initErr error
	geoIPOnce.Do(func() {
		db, err := geoip2.Open(dbPath)
		if err != nil {
			logger.Error(ctx, "geoip: failed to open database",
				zap.String("path", dbPath),
				zap.Error(err),
			)
			initErr = err
			return
		}
		geoIPInstance = &GeoIPLookup{db: db}
	})
	return initErr
}

func CloseGeoIP() {
	if geoIPInstance != nil && geoIPInstance.db != nil {
		geoIPInstance.db.Close()
	}
}

func LookupIP(ctx context.Context, ipStr string) GeoIPResult {
	if geoIPInstance == nil || geoIPInstance.db == nil {
		return GeoIPResult{}
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return GeoIPResult{}
	}

	geoIPInstance.mu.RLock()
	defer geoIPInstance.mu.RUnlock()

	record, err := geoIPInstance.db.City(ip)
	if err != nil {
		logger.Warn(ctx, "geoip: failed to lookup IP",
			zap.String("ip", ipStr),
			zap.Error(err),
		)
		return GeoIPResult{}
	}

	return GeoIPResult{
		Country: record.Country.IsoCode,
		City:    record.City.Names["en"],
	}
}
