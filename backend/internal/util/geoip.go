package util

import (
	"net"
	"sync"

	"github.com/oschwald/geoip2-golang"
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

func InitGeoIP(dbPath string) error {
	if dbPath == "" {
		return nil // GeoIP disabled
	}

	var initErr error
	geoIPOnce.Do(func() {
		db, err := geoip2.Open(dbPath)
		if err != nil {
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

func LookupIP(ipStr string) GeoIPResult {
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
		return GeoIPResult{}
	}

	return GeoIPResult{
		Country: record.Country.IsoCode,
		City:    record.City.Names["en"],
	}
}
