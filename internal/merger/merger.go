package merger

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"runtime"
	"time"

	"merged-ip-data/internal/config"
	"merged-ip-data/internal/interner"
	"merged-ip-data/internal/reader"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
)

const maxMergeWorkers = 8

// logMemStats logs current memory statistics for profiling
func logMemStats(phase string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("[Memory] %s: Alloc=%d MB, TotalAlloc=%d MB, Sys=%d MB, NumGC=%d\n",
		phase,
		m.Alloc/1024/1024,
		m.TotalAlloc/1024/1024,
		m.Sys/1024/1024,
		m.NumGC)
}

func mergeWorkerCount() int {
	numWorkers := runtime.GOMAXPROCS(0)
	if numWorkers < 1 {
		return 1
	}
	if numWorkers > maxMergeWorkers {
		return maxMergeWorkers
	}
	return numWorkers
}

// closerList holds a list of io.Closers for cleanup
type closerList []io.Closer

// closeAll closes all resources and returns the first error encountered
func (cl closerList) closeAll() error {
	var firstErr error
	for _, c := range cl {
		if c != nil {
			if err := c.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// Merger handles the merging of multiple IP databases
type Merger struct {
	geoLiteCity     *reader.GeoLite2CityReader
	geoLiteASN      *reader.GeoLite2ASNReader
	ipinfoLite      *reader.IPinfoLiteReader
	dbipCity        *reader.DBIPCityReader
	routeViewsASN   *reader.RouteViewsASNReader
	geoWhoisCountry *reader.GeoWhoisCountryReader
	qqwry           *reader.QQWryReader
	openproxyDB     *reader.OpenproxyDBReader
	badASN          *reader.BadASNReader

	tree *mmdbwriter.Tree

	stats Stats

	// Reusable records for lookups to reduce allocations during merge
	reusableIPinfoRecord      reader.IPinfoLiteRecord
	reusableGeoLiteASNRecord  reader.GeoLite2ASNRecord
	reusableRouteViewsRecord  reader.RouteViewsASNRecord
	reusableGeoWhoisRecord    reader.GeoWhoisCountryRecord
	reusableQQWryRecord       reader.QQWryRecord
	reusableGeoLiteCityRecord reader.GeoLite2CityRecord
	reusableOpenproxyDBRecord reader.OpenproxyDBRecord

	// ASN lookup cache to avoid redundant lookups for adjacent networks
	cachedASN        ASNRecord
	cachedASNNetwork *net.IPNet
	cachedASNSource  asnSource
	cachedASNValid   bool
}

// Stats holds merge statistics
type Stats struct {
	TotalNetworks                    int64
	GeoLiteCityHits                  int64
	GeoLiteASNHits                   int64
	IPinfoLiteHits                   int64
	DBIPHits                         int64
	RouteViewsASNHits                int64
	GeoWhoisCountryHits              int64
	QQWryHits                        int64
	OpenproxyDBHits                  int64
	OpenproxyDBCIDRRangesInserted    int64
	BadASNHits                       int64
	EmptyRecords                     int64
	ProcessedNetworks                int64
	SingleProxyIPsInserted           int64
	ICloudPrivateRelayRangesInserted int64
	AnycastPrefixesInserted          int64
}

type asnSource uint8

const (
	asnSourceNone asnSource = iota
	asnSourceIPinfo
	asnSourceGeoLite
	asnSourceRouteViews
)

// New creates a new Merger instance
func New() (*Merger, error) {
	// Initialize string interner with common values
	interner.Init()

	var closers closerList
	cleanup := func() { closers.closeAll() }

	geoLiteCity, err := reader.OpenGeoLite2City()
	if err != nil {
		return nil, fmt.Errorf("failed to open GeoLite2-City: %w", err)
	}
	closers = append(closers, geoLiteCity)

	geoLiteASN, err := reader.OpenGeoLite2ASN()
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to open GeoLite2-ASN: %w", err)
	}
	closers = append(closers, geoLiteASN)

	ipinfoLite, err := reader.OpenIPinfoLite()
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to open IPinfo Lite: %w", err)
	}
	closers = append(closers, ipinfoLite)

	dbipCity, err := reader.OpenDBIPCity()
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to open DB-IP City: %w", err)
	}
	closers = append(closers, dbipCity)

	routeViewsASN, err := reader.OpenRouteViewsASN()
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to open RouteViews ASN: %w", err)
	}
	closers = append(closers, routeViewsASN)

	geoWhoisCountry, err := reader.OpenGeoWhoisCountry()
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to open GeoWhois Country: %w", err)
	}
	closers = append(closers, geoWhoisCountry)

	qqwry, err := reader.OpenQQWry()
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to open QQWry: %w", err)
	}
	closers = append(closers, qqwry)

	openproxyDB, err := reader.OpenOpenproxyDB()
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to open OpenProxyDB: %w", err)
	}
	closers = append(closers, openproxyDB)

	badASN, err := reader.OpenBadASNList(config.BadASNListFile)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to open bad ASN list: %w", err)
	}
	closers = append(closers, badASN)
	fmt.Printf("Bad ASN list loaded: %d ASNs (includes %d manual entries)\n",
		badASN.Count(), len(reader.ManuallyAddedBadASNs))

	singleIPs, cidrRanges := openproxyDB.Stats()
	fmt.Printf("OpenProxyDB loaded: %d single IPs, %d CIDR ranges\n", singleIPs, cidrRanges)

	badIPCount, err := openproxyDB.LoadBadIPList(config.BadIPListFile)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to load BadIPList: %w", err)
	}
	fmt.Printf("BadIPList loaded: %d IPs merged into proxy data\n", badIPCount)

	torCount, err := openproxyDB.LoadTorRelays(config.TorRelaysFile)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to load Tor relays: %w", err)
	}
	fmt.Printf("Tor relays loaded: %d unique IPs merged into proxy data\n", torCount)

	icloudCount, err := openproxyDB.LoadICloudPrivateRelayRanges(config.ICloudPrivateRelayFile)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to load iCloud Private Relay ranges: %w", err)
	}
	fmt.Printf("iCloud Private Relay ranges loaded: %d CIDRs merged into proxy/VPN data\n", icloudCount)

	anycastCount, err := openproxyDB.LoadAnycastPrefixes(config.AnycastV4File, config.AnycastV6File)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to load anycast prefixes: %w", err)
	}
	fmt.Printf("Anycast prefixes loaded: %d entries (%d in lookup set) — CDN overlay active\n",
		anycastCount, openproxyDB.AnycastPrefixCount())

	singleIPs, cidrRanges = openproxyDB.Stats()
	fmt.Printf("OpenProxyDB total after merge: %d single IPs, %d CIDR ranges\n", singleIPs, cidrRanges)

	tree, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType:            config.DatabaseType,
		Description:             map[string]string{"en": config.DatabaseDescription},
		Languages:               config.SupportedLanguages,
		IPVersion:               6,
		RecordSize:              28,
		IncludeReservedNetworks: false,
		DisableIPv4Aliasing:     false,
	})
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to create mmdb tree: %w", err)
	}

	return &Merger{
		geoLiteCity:     geoLiteCity,
		geoLiteASN:      geoLiteASN,
		ipinfoLite:      ipinfoLite,
		dbipCity:        dbipCity,
		routeViewsASN:   routeViewsASN,
		geoWhoisCountry: geoWhoisCountry,
		qqwry:           qqwry,
		openproxyDB:     openproxyDB,
		badASN:          badASN,
		tree:            tree,
	}, nil
}

// Close closes all database readers
func (m *Merger) Close() error {
	var errs []error

	if m.geoLiteCity != nil {
		if err := m.geoLiteCity.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if m.geoLiteASN != nil {
		if err := m.geoLiteASN.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if m.ipinfoLite != nil {
		if err := m.ipinfoLite.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if m.dbipCity != nil {
		if err := m.dbipCity.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if m.routeViewsASN != nil {
		if err := m.routeViewsASN.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if m.geoWhoisCountry != nil {
		if err := m.geoWhoisCountry.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if m.qqwry != nil {
		if err := m.qqwry.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if m.openproxyDB != nil {
		if err := m.openproxyDB.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if m.badASN != nil {
		if err := m.badASN.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing readers: %v", errs)
	}
	return nil
}

// Merge performs the database merge operation
func (m *Merger) Merge() error {
	fmt.Println("Starting database merge...")
	startTime := time.Now()
	logMemStats("Start")

	numWorkers := mergeWorkerCount()
	fmt.Printf("Processing GeoLite2-City networks (primary source) with %d workers...\n", numWorkers)
	if err := m.processGeoLiteCityNetworksParallel(numWorkers); err != nil {
		return fmt.Errorf("failed to process GeoLite2-City: %w", err)
	}
	logMemStats("After GeoLite2-City")

	// Release memory from completed phase before starting next
	runtime.GC()
	logMemStats("After GC (Phase 1)")

	fmt.Println("Processing DB-IP networks (supplementary data)...")
	if err := m.processDBIPNetworks(); err != nil {
		return fmt.Errorf("failed to process DB-IP: %w", err)
	}
	logMemStats("After DB-IP")

	fmt.Println("Processing OpenProxyDB CIDR ranges (direct CIDR insertion)...")
	if err := m.processOpenProxyDBCIDRRanges(); err != nil {
		return fmt.Errorf("failed to process OpenProxyDB CIDR ranges: %w", err)
	}
	logMemStats("After OpenProxyDB CIDR ranges")

	fmt.Println("Processing iCloud Private Relay ranges (direct CIDR insertion)...")
	if err := m.processICloudPrivateRelayRanges(); err != nil {
		return fmt.Errorf("failed to process iCloud Private Relay ranges: %w", err)
	}
	logMemStats("After iCloud Private Relay")

	fmt.Println("Processing anycast prefixes (direct CDN insertion)...")
	if err := m.processAnycastPrefixes(); err != nil {
		return fmt.Errorf("failed to process anycast prefixes: %w", err)
	}
	logMemStats("After Anycast Prefixes")

	fmt.Println("Processing single proxy IPs (direct /32 and /128 insertion)...")
	if err := m.processSingleProxyIPs(); err != nil {
		return fmt.Errorf("failed to process single proxy IPs: %w", err)
	}
	logMemStats("After Single Proxy IPs")

	// Final GC before write phase
	runtime.GC()
	logMemStats("After GC (Phase 3)")

	elapsed := time.Since(startTime)
	fmt.Printf("Merge completed in %v\n", elapsed)
	m.printStats()

	// Print interner statistics
	fmt.Printf("[Interner] %s\n", interner.Stats())

	return nil
}

// processGeoLiteCityNetworksParallel processes GeoLite2-City networks using parallel workers.
// This significantly speeds up processing on multi-core systems by:
// 1. Reading networks from GeoLite2-City sequentially (iterator is not thread-safe)
// 2. Processing enrichment (ASN, QQWry, etc.) in parallel via worker pool
// 3. Inserting results into the tree sequentially (tree is not thread-safe)
func (m *Merger) processGeoLiteCityNetworksParallel(numWorkers int) error {
	// Create worker pool
	pool := newWorkerPool(
		numWorkers,
		m.ipinfoLite,
		m.geoLiteASN,
		m.routeViewsASN,
		m.geoWhoisCountry,
		m.qqwry,
		m.openproxyDB,
		m.badASN,
	)

	// Start workers
	pool.start()

	// Start result consumer in a separate goroutine
	var insertErr error
	var insertedCount int64
	insertDone := make(chan struct{})

	go func() {
		defer close(insertDone)
		for result := range pool.results() {
			if err := m.tree.Insert(result.network, result.mmdbRecord); err != nil {
				if isSkippableInsertError(err) {
					continue
				}
				if insertErr == nil {
					insertErr = fmt.Errorf("failed to insert network %s: %w", result.network, err)
				}
				fmt.Printf("Warning: failed to insert network %s: %v\n", result.network, err)
				continue
			}
			insertedCount++
			if insertedCount%100000 == 0 {
				fmt.Printf("  Inserted %d networks...\n", insertedCount)
			}
		}
	}()

	// Read networks and submit to worker pool
	networks := m.geoLiteCity.Networks()
	for networks.Next() {
		var geoRecord reader.GeoLite2CityRecord
		network, err := networks.Network(&geoRecord)
		if err != nil {
			fmt.Printf("Warning: failed to read network: %v\n", err)
			continue
		}

		pool.submit(workItem{
			network:   network,
			geoRecord: geoRecord,
		})
	}

	// Signal no more work
	pool.closeWork()

	// Wait for all workers to finish
	pool.wait()

	// Wait for all insertions to complete
	<-insertDone

	// Check for iterator errors
	if err := networks.Err(); err != nil {
		return err
	}

	if insertErr != nil {
		return insertErr
	}

	// Aggregate statistics from workers
	workerStats := pool.aggregateStats()
	m.stats.TotalNetworks = workerStats.TotalNetworks
	m.stats.GeoLiteCityHits = workerStats.GeoLiteCityHits
	m.stats.GeoLiteASNHits = workerStats.GeoLiteASNHits
	m.stats.IPinfoLiteHits = workerStats.IPinfoLiteHits
	m.stats.RouteViewsASNHits = workerStats.RouteViewsASNHits
	m.stats.GeoWhoisCountryHits = workerStats.GeoWhoisCountryHits
	m.stats.QQWryHits = workerStats.QQWryHits
	m.stats.OpenproxyDBHits = workerStats.OpenproxyDBHits
	m.stats.BadASNHits = workerStats.BadASNHits
	m.stats.EmptyRecords = workerStats.EmptyRecords
	m.stats.ProcessedNetworks = insertedCount

	return nil
}

// processDBIPNetworks processes DB-IP networks for IPs not covered by GeoLite2
func (m *Merger) processDBIPNetworks() error {
	if err := m.processDBIPReader(m.dbipCity.IPv4Reader()); err != nil {
		return err
	}
	return m.processDBIPReader(m.dbipCity.IPv6Reader())
}

func (m *Merger) processDBIPReader(r *reader.Reader) error {
	networks := r.Networks()

	// Reuse a single record to reduce allocations
	var record MergedRecord

	for networks.Next() {
		var dbipRecord reader.DBIPCityRecord
		network, err := networks.Network(&dbipRecord)
		if err != nil {
			fmt.Printf("Warning: failed to read DB-IP network: %v\n", err)
			continue
		}

		if !dbipRecord.HasGeoData() {
			continue
		}

		ip := network.IP

		// Use reusable record to check if GeoLite2 has data for this IP
		m.reusableGeoLiteCityRecord.Reset()
		if err := m.geoLiteCity.LookupTo(ip, &m.reusableGeoLiteCityRecord); err == nil && m.reusableGeoLiteCityRecord.HasPrimaryGeoData() {
			continue
		}

		m.stats.TotalNetworks++

		record.Reset()
		m.buildMergedRecordFromDBIP(network, &dbipRecord, &record)

		if record.IsEmpty() {
			m.stats.EmptyRecords++
			continue
		}

		if err := m.insertWithMerge(network, &record); err != nil {
			// Silently skip reserved and aliased networks - these are expected
			// when DB-IP data contains IANA special-purpose address ranges
			if isSkippableInsertError(err) {
				continue
			}
			fmt.Printf("Warning: failed to insert DB-IP network %s: %v\n", network, err)
			continue
		}

		m.stats.DBIPHits++
		m.stats.ProcessedNetworks++
	}

	return networks.Err()
}

// buildMergedRecordFromDBIP creates a merged record using DB-IP as primary geo source.
// The record parameter should be pre-reset before calling this function.
func (m *Merger) buildMergedRecordFromDBIP(network *net.IPNet, dbipRecord *reader.DBIPCityRecord, record *MergedRecord) {
	if dbipRecord.HasGeoData() {
		if dbipRecord.City != "" {
			record.City = CityRecord{
				Names: map[string]string{"en": dbipRecord.City},
			}
		}

		if dbipRecord.CountryCode != "" {
			record.Country = CountryRecord{
				ISOCode: dbipRecord.CountryCode,
			}
		}

		if dbipRecord.HasLocationData() {
			latitude, longitude, hasCoordinates := dbipRecord.Coordinates()
			record.Location = LocationRecord{
				Latitude:       latitude,
				Longitude:      longitude,
				TimeZone:       dbipRecord.Timezone,
				HasCoordinates: hasCoordinates,
			}
		}

		if dbipRecord.Postcode != "" {
			record.Postal = PostalRecord{
				Code: dbipRecord.Postcode,
			}
		}

		if dbipRecord.State1 != "" {
			record.Subdivisions = []SubdivisionRecord{
				{
					Names: map[string]string{"en": dbipRecord.State1},
				},
			}
		}
	}

	m.enrichWithASNData(network.IP, record)
	m.enrichWithCountryFallback(network.IP, record)
	m.enrichWithQQWryData(network.IP, record)
	m.enrichWithProxyData(network.IP, record)
}

// enrichWithCountryFallback adds country information from GeoWhois when country is missing
func (m *Merger) enrichWithCountryFallback(ip net.IP, record *MergedRecord) {
	if record.Country.ISOCode != "" {
		return
	}

	m.reusableGeoWhoisRecord.Reset()
	if err := m.geoWhoisCountry.LookupTo(ip, &m.reusableGeoWhoisRecord); err == nil && m.reusableGeoWhoisRecord.HasCountry() {
		m.stats.GeoWhoisCountryHits++
		record.Country.ISOCode = m.reusableGeoWhoisRecord.CountryCode
	}
}

// enrichWithQQWryData adds Chinese location data from QQWry (Chunzhen) database for Chinese IPs.
// This provides more accurate and detailed Chinese location names (zh-CN) for IPs in China.
func (m *Merger) enrichWithQQWryData(ip net.IP, record *MergedRecord) {
	// Only enrich for Chinese IPs
	if record.Country.ISOCode != "CN" {
		return
	}

	m.reusableQQWryRecord.Reset()
	if err := m.qqwry.LookupTo(ip, &m.reusableQQWryRecord); err != nil || !m.reusableQQWryRecord.HasGeoData() {
		return
	}

	// Verify the record is indeed for China
	if !m.reusableQQWryRecord.IsChina() {
		return
	}

	m.stats.QQWryHits++

	// Enrich city names with Chinese (zh-CN)
	if m.reusableQQWryRecord.HasCityData() {
		record.City.Names = withName(record.City.Names, "zh-CN", m.reusableQQWryRecord.CityName)
	}

	// Enrich subdivision (province) names with Chinese (zh-CN)
	if m.reusableQQWryRecord.HasRegionData() {
		if len(record.Subdivisions) == 0 {
			record.Subdivisions = []SubdivisionRecord{{
				Names: map[string]string{"zh-CN": m.reusableQQWryRecord.RegionName},
			}}
		} else {
			record.Subdivisions[0].Names = withName(record.Subdivisions[0].Names, "zh-CN", m.reusableQQWryRecord.RegionName)
		}
	}

	// Add Chinese country name if not present
	if m.reusableQQWryRecord.CountryName != "" {
		if _, ok := record.Country.Names["zh-CN"]; !ok {
			record.Country.Names = withName(record.Country.Names, "zh-CN", m.reusableQQWryRecord.CountryName)
		}
	}
}

// enrichWithASNData adds ASN information from IPinfo Lite (primary), GeoLite2-ASN (secondary), or RouteViews (tertiary).
// Uses caching to avoid redundant lookups for IPs within the same ASN network.
func (m *Merger) enrichWithASNData(ip net.IP, record *MergedRecord) {
	// Check cache first - if IP is within cached ASN network, reuse the result
	if m.cachedASNValid && m.cachedASNNetwork != nil && m.cachedASNNetwork.Contains(ip) {
		if m.cachedASN.Number != 0 {
			record.ASN = m.cachedASN
			m.incrementASNHit(m.cachedASNSource)
		}
		return
	}

	// Cache miss - perform lookups
	m.cachedASNValid = false
	m.cachedASNNetwork = nil
	m.cachedASNSource = asnSourceNone

	// Priority 1: IPinfo Lite (includes as_domain)
	m.reusableIPinfoRecord.Reset()
	if network, lookupOK, err := m.ipinfoLite.LookupNetworkTo(ip, &m.reusableIPinfoRecord); err == nil && lookupOK && m.reusableIPinfoRecord.HasASN() {
		m.stats.IPinfoLiteHits++
		record.ASN = ASNRecord{
			Number:       m.reusableIPinfoRecord.GetASNumber(),
			Organization: m.reusableIPinfoRecord.ASName,
			Domain:       m.reusableIPinfoRecord.ASDomain,
		}
		m.cachedASN = record.ASN
		m.cachedASNNetwork = network
		m.cachedASNSource = asnSourceIPinfo
		m.cachedASNValid = true
		return
	}

	// Priority 2: GeoLite2-ASN
	m.reusableGeoLiteASNRecord.Reset()
	if network, lookupOK, err := m.geoLiteASN.LookupNetworkTo(ip, &m.reusableGeoLiteASNRecord); err == nil && lookupOK && m.reusableGeoLiteASNRecord.HasASN() {
		m.stats.GeoLiteASNHits++
		record.ASN = ASNRecord{
			Number:       m.reusableGeoLiteASNRecord.AutonomousSystemNumber,
			Organization: m.reusableGeoLiteASNRecord.AutonomousSystemOrganization,
		}
		m.cachedASN = record.ASN
		m.cachedASNNetwork = network
		m.cachedASNSource = asnSourceGeoLite
		m.cachedASNValid = true
		return
	}

	// Priority 3: RouteViews ASN
	m.reusableRouteViewsRecord.Reset()
	if network, lookupOK, err := m.routeViewsASN.LookupNetworkTo(ip, &m.reusableRouteViewsRecord); err == nil && lookupOK && m.reusableRouteViewsRecord.HasASN() {
		m.stats.RouteViewsASNHits++
		record.ASN = ASNRecord{
			Number:       m.reusableRouteViewsRecord.AutonomousSystemNumber,
			Organization: m.reusableRouteViewsRecord.AutonomousSystemOrganization,
		}
		m.cachedASN = record.ASN
		m.cachedASNNetwork = network
		m.cachedASNSource = asnSourceRouteViews
		m.cachedASNValid = true
		return
	}

	// No ASN found - cache the miss with empty record
	m.cachedASN = ASNRecord{}
	m.cachedASNSource = asnSourceNone
	m.cachedASNValid = true
}

func (m *Merger) incrementASNHit(source asnSource) {
	switch source {
	case asnSourceIPinfo:
		m.stats.IPinfoLiteHits++
	case asnSourceGeoLite:
		m.stats.GeoLiteASNHits++
	case asnSourceRouteViews:
		m.stats.RouteViewsASNHits++
	}
}

// enrichWithProxyData adds proxy/anonymity information from OpenProxyDB, with
// a bad-ASN fallback: if OpenProxyDB did not flag the IP as a proxy but the
// ASN resolved earlier is in the bad ASN list, overlay IsProxy/IsHosting/
// IsAnonymous onto whatever proxy record is already present.
func (m *Merger) enrichWithProxyData(ip net.IP, record *MergedRecord) {
	m.reusableOpenproxyDBRecord.Reset()
	if m.openproxyDB.LookupTo(ip, &m.reusableOpenproxyDBRecord) {
		m.stats.OpenproxyDBHits++
		record.Proxy = ProxyRecord{
			IsProxy:     m.reusableOpenproxyDBRecord.IsProxy,
			IsVPN:       m.reusableOpenproxyDBRecord.IsVPN,
			IsTor:       m.reusableOpenproxyDBRecord.IsTor,
			IsHosting:   m.reusableOpenproxyDBRecord.IsHosting,
			IsCDN:       m.reusableOpenproxyDBRecord.IsCDN,
			IsSchool:    m.reusableOpenproxyDBRecord.IsSchool,
			IsAnonymous: m.reusableOpenproxyDBRecord.IsAnonymous,
		}
	}

	applySchoolASNMatch(record)

	if !record.Proxy.IsProxy && record.ASN.Number != 0 && m.badASN.Contains(record.ASN.Number) {
		m.stats.BadASNHits++
		record.Proxy.IsProxy = true
		record.Proxy.IsHosting = true
		record.Proxy.IsAnonymous = true
	}
}

// processICloudPrivateRelayRanges directly overlays every Apple-published
// iCloud Private Relay CIDR with proxy and VPN flags.
func (m *Merger) processICloudPrivateRelayRanges() error {
	ranges := m.openproxyDB.ICloudPrivateRelayRanges()
	if len(ranges) == 0 {
		fmt.Println("iCloud Private Relay ranges: 0 inserted, 0 skipped")
		return nil
	}

	proxy := ProxyRecord{
		IsProxy:     true,
		IsVPN:       true,
		IsAnonymous: true,
	}
	proxyMMDB := proxy.toMMDBType()

	inserted := 0
	skipped := 0
	for _, prefix := range ranges {
		network := netipPrefixToIPNet(prefix)
		if network == nil {
			skipped++
			continue
		}

		if err := m.insertProxyMap(network, proxyMMDB); err != nil {
			if isSkippableInsertError(err) {
				skipped++
				continue
			}
			fmt.Printf("Warning: failed to insert iCloud Private Relay range %s: %v\n", prefix, err)
			skipped++
			continue
		}
		inserted++
	}

	fmt.Printf("iCloud Private Relay ranges: %d inserted, %d skipped (of %d total)\n", inserted, skipped, len(ranges))
	m.stats.ICloudPrivateRelayRangesInserted = int64(inserted)
	return nil
}

// processOpenProxyDBCIDRRanges directly overlays every CIDR range from
// OpenProxyDB. Lookup-time enrichment only samples a source network's base IP,
// so direct insertion is required for exact proxy coverage when proxy ranges
// are narrower than the geo/ASN networks already in the tree.
func (m *Merger) processOpenProxyDBCIDRRanges() error {
	ranges := m.openproxyDB.CIDRRanges()
	if len(ranges) == 0 {
		fmt.Println("OpenProxyDB CIDR ranges: 0 inserted, 0 skipped")
		return nil
	}

	inserted := 0
	skipped := 0
	for _, cidrRange := range ranges {
		proxy := ProxyRecord{
			IsProxy:     cidrRange.Record.IsProxy,
			IsVPN:       cidrRange.Record.IsVPN,
			IsTor:       cidrRange.Record.IsTor,
			IsHosting:   cidrRange.Record.IsHosting,
			IsCDN:       cidrRange.Record.IsCDN,
			IsSchool:    cidrRange.Record.IsSchool,
			IsAnonymous: cidrRange.Record.IsAnonymous,
		}
		proxyMMDB := proxy.toMMDBType()
		if proxyMMDB == nil {
			skipped++
			continue
		}

		network := netipPrefixToIPNet(cidrRange.Prefix)
		if network == nil {
			skipped++
			continue
		}

		if err := m.insertProxyMap(network, proxyMMDB); err != nil {
			if isSkippableInsertError(err) {
				skipped++
				continue
			}
			fmt.Printf("Warning: failed to insert OpenProxyDB CIDR range %s: %v\n", cidrRange.Prefix, err)
			skipped++
			continue
		}
		inserted++
	}

	fmt.Printf("OpenProxyDB CIDR ranges: %d inserted, %d skipped (of %d total)\n", inserted, skipped, len(ranges))
	m.stats.OpenproxyDBCIDRRangesInserted = int64(inserted)
	return nil
}

// processAnycastPrefixes directly overlays bgp.tools anycast prefixes with
// the CDN flag. This avoids relying on geo/ASN source network boundaries to
// happen to align with anycast CIDRs.
func (m *Merger) processAnycastPrefixes() error {
	prefixes := m.openproxyDB.AnycastPrefixes()
	if len(prefixes) == 0 {
		fmt.Println("Anycast prefixes: 0 inserted, 0 skipped")
		return nil
	}

	proxyMMDB := (&ProxyRecord{IsCDN: true}).toMMDBType()
	inserted := 0
	skipped := 0
	for _, prefix := range prefixes {
		network := netipPrefixToIPNet(prefix)
		if network == nil {
			skipped++
			continue
		}

		if err := m.insertProxyMap(network, proxyMMDB); err != nil {
			if isSkippableInsertError(err) {
				skipped++
				continue
			}
			fmt.Printf("Warning: failed to insert anycast prefix %s: %v\n", prefix, err)
			skipped++
			continue
		}
		inserted++
	}

	fmt.Printf("Anycast prefixes: %d inserted, %d skipped (of %d total)\n", inserted, skipped, len(prefixes))
	m.stats.AnycastPrefixesInserted = int64(inserted)
	return nil
}

// processSingleProxyIPs directly inserts every single IP from OpenProxyDB and BadIPList
// as /32 (IPv4) or /128 (IPv6) networks into the MMDB tree.
// This ensures complete proxy coverage for individual IPs that would otherwise be missed
// when only the network base address is checked during enrichment.
// Uses InsertFunc to merge proxy flags with any existing geo/ASN data in the tree.
func (m *Merger) processSingleProxyIPs() error {
	singleIPs := m.openproxyDB.SingleIPs()
	inserted := 0
	skipped := 0

	for addr, proxyRecord := range singleIPs {
		// Build the proxy mmdbtype
		proxy := ProxyRecord{
			IsProxy:     proxyRecord.IsProxy,
			IsVPN:       proxyRecord.IsVPN,
			IsTor:       proxyRecord.IsTor,
			IsHosting:   proxyRecord.IsHosting,
			IsCDN:       proxyRecord.IsCDN,
			IsSchool:    proxyRecord.IsSchool,
			IsAnonymous: proxyRecord.IsAnonymous,
		}
		proxyMMDB := proxy.toMMDBType()
		if proxyMMDB == nil {
			skipped++
			continue
		}

		// Convert netip.Addr to net.IP and build /32 or /128 network
		ip := addr.AsSlice()
		ones := 32
		if addr.Is6() {
			ones = 128
		}
		network := &net.IPNet{
			IP:   ip,
			Mask: net.CIDRMask(ones, ones),
		}

		if err := m.insertProxyMap(network, proxyMMDB); err != nil {
			// Silently skip reserved and aliased networks — consistent with DB-IP phase
			if isSkippableInsertError(err) {
				skipped++
				continue
			}
			fmt.Printf("Warning: failed to insert single proxy IP %s: %v\n", addr, err)
			skipped++
			continue
		}
		inserted++
	}

	fmt.Printf("Single proxy IPs: %d inserted, %d skipped (of %d total)\n", inserted, skipped, len(singleIPs))
	m.stats.SingleProxyIPsInserted = int64(inserted)
	return nil
}

func netipPrefixToIPNet(prefix netip.Prefix) *net.IPNet {
	if !prefix.IsValid() {
		return nil
	}

	prefix = prefix.Masked()
	addr := prefix.Addr()
	bits := prefix.Bits()
	if addr.Is4In6() {
		if bits >= 96 {
			bits -= 96
		} else {
			bits = 0
		}
	}
	addr = addr.Unmap()
	bitLen := addr.BitLen()
	if bits > bitLen {
		bits = bitLen
	}

	return &net.IPNet{
		IP:   addr.AsSlice(),
		Mask: net.CIDRMask(bits, bitLen),
	}
}

func (m *Merger) insertProxyMap(network *net.IPNet, proxyMMDB mmdbtype.Map) error {
	return m.tree.InsertFunc(network, func(existing mmdbtype.DataType) (mmdbtype.DataType, error) {
		if existing == nil {
			return mmdbtype.Map{keyProxy: proxyMMDB}, nil
		}

		existingMap, ok := existing.(mmdbtype.Map)
		if !ok {
			return mmdbtype.Map{keyProxy: proxyMMDB}, nil
		}

		copied := existingMap.Copy().(mmdbtype.Map)
		if prev, hasPrev := copied[keyProxy].(mmdbtype.Map); hasPrev {
			copied[keyProxy] = unionProxyMaps(prev, proxyMMDB)
		} else {
			copied[keyProxy] = proxyMMDB
		}
		return copied, nil
	})
}

func isSkippableInsertError(err error) bool {
	var aliasedErr *mmdbwriter.AliasedNetworkError
	var reservedErr *mmdbwriter.ReservedNetworkError
	return errors.As(err, &aliasedErr) || errors.As(err, &reservedErr)
}

// insertWithMerge inserts a record, merging with existing data if present
func (m *Merger) insertWithMerge(network *net.IPNet, record *MergedRecord) error {
	return m.tree.InsertFunc(network, func(existing mmdbtype.DataType) (mmdbtype.DataType, error) {
		if existing == nil {
			return record.ToMMDBType(), nil
		}

		existingMap, ok := existing.(mmdbtype.Map)
		if !ok {
			return record.ToMMDBType(), nil
		}

		newMap := record.ToMMDBType()
		return mergeMMDBMaps(existingMap, newMap), nil
	})
}

// mergeMMDBMaps merges two mmdbtype.Map values, with new values filling in missing fields
func mergeMMDBMaps(existing, new mmdbtype.Map) mmdbtype.Map {
	result := mmdbtype.Map{}

	for k, v := range existing {
		result[k] = v
	}

	for k, v := range new {
		existingValue, exists := result[k]
		if !exists {
			result[k] = v
			continue
		}
		if k == keyProxy {
			if existingProxy, ok := existingValue.(mmdbtype.Map); ok {
				if newProxy, ok := v.(mmdbtype.Map); ok {
					result[k] = unionProxyMaps(existingProxy, newProxy)
				}
			}
			continue
		}
		if existingMap, ok := existingValue.(mmdbtype.Map); ok {
			if newMap, ok := v.(mmdbtype.Map); ok {
				result[k] = mergeMMDBMaps(existingMap, newMap)
			}
		}
	}

	return result
}

// unionProxyMaps returns a fresh map containing the union of boolean flag keys
// from a and b. Every value in the inputs is a mmdbtype.Bool(true) (the proxy
// encoder omits false flags), so a key present in either map means "true".
// Neither input is mutated.
func unionProxyMaps(a, b mmdbtype.Map) mmdbtype.Map {
	result := mmdbtype.Map{}
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}

// Tree returns the mmdbwriter tree for writing
func (m *Merger) Tree() *mmdbwriter.Tree {
	return m.tree
}

// Stats returns the merge statistics
func (m *Merger) Stats() Stats {
	return m.stats
}

func (m *Merger) printStats() {
	fmt.Println("Merge Statistics:")
	fmt.Printf("  Total networks processed: %d\n", m.stats.TotalNetworks)
	fmt.Printf("  GeoLite2-City hits: %d\n", m.stats.GeoLiteCityHits)
	fmt.Printf("  GeoLite2-ASN hits: %d\n", m.stats.GeoLiteASNHits)
	fmt.Printf("  IPinfo Lite hits: %d\n", m.stats.IPinfoLiteHits)
	fmt.Printf("  RouteViews ASN hits: %d\n", m.stats.RouteViewsASNHits)
	fmt.Printf("  DB-IP supplementary records: %d\n", m.stats.DBIPHits)
	fmt.Printf("  GeoWhois Country fallback hits: %d\n", m.stats.GeoWhoisCountryHits)
	fmt.Printf("  QQWry (Chunzhen) China enrichment hits: %d\n", m.stats.QQWryHits)
	fmt.Printf("  OpenProxyDB proxy enrichment hits: %d\n", m.stats.OpenproxyDBHits)
	fmt.Printf("  OpenProxyDB CIDR ranges inserted: %d\n", m.stats.OpenproxyDBCIDRRangesInserted)
	fmt.Printf("  Bad ASN fallback hits: %d\n", m.stats.BadASNHits)
	fmt.Printf("  iCloud Private Relay ranges inserted: %d\n", m.stats.ICloudPrivateRelayRangesInserted)
	fmt.Printf("  Anycast prefixes inserted: %d\n", m.stats.AnycastPrefixesInserted)
	fmt.Printf("  Single proxy IPs inserted (/32, /128): %d\n", m.stats.SingleProxyIPsInserted)
	fmt.Printf("  Empty records skipped: %d\n", m.stats.EmptyRecords)
	fmt.Printf("  Final network count: %d\n", m.stats.ProcessedNetworks)
}
