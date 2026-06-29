package reader

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ASNOverlaySource identifies an ASN list and the proxy flags that should be
// applied to IPs resolving to ASNs in that list.
type ASNOverlaySource struct {
	Path   string
	Record OpenproxyDBRecord
}

// ASNOverlayReader holds ASN-based proxy flag overlays.
type ASNOverlayReader struct {
	records map[uint32]OpenproxyDBRecord
}

// OpenASNOverlayLists loads one or more ASN lists and merges duplicate ASN
// records by OR'ing their flags.
func OpenASNOverlayLists(sources ...ASNOverlaySource) (*ASNOverlayReader, error) {
	r := &ASNOverlayReader{
		records: make(map[uint32]OpenproxyDBRecord),
	}

	for _, source := range sources {
		if _, err := r.Load(source.Path, source.Record); err != nil {
			return nil, err
		}
	}

	return r, nil
}

// Load reads a plain-text ASN list. Lines may contain inline '#' comments.
func (r *ASNOverlayReader) Load(path string, record OpenproxyDBRecord) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("failed to open ASN overlay file %s: %w", path, err)
	}
	defer file.Close()

	if r.records == nil {
		r.records = make(map[uint32]OpenproxyDBRecord)
	}
	record = normalizeOpenproxyRecord(record)

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	count := 0
	for scanner.Scan() {
		asn, ok := parsePlainASNLine(scanner.Text())
		if !ok {
			continue
		}
		if existing, found := r.records[asn]; found {
			r.records[asn] = mergeOpenproxyRecords(existing, record)
		} else {
			r.records[asn] = record
		}
		count++
	}

	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("error reading ASN overlay file %s: %w", path, err)
	}

	return count, nil
}

func parsePlainASNLine(line string) (uint32, bool) {
	if comment := strings.Index(line, "#"); comment >= 0 {
		line = line[:comment]
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return 0, false
	}

	token := strings.Trim(fields[0], ",;")
	return parseASNField(token)
}

func normalizeOpenproxyRecord(record OpenproxyDBRecord) OpenproxyDBRecord {
	if record.IsProxy || record.IsVPN || record.IsTor {
		record.IsAnonymous = true
	}
	return record
}

// Lookup returns the proxy flags for an ASN.
func (r *ASNOverlayReader) Lookup(asn uint32) (OpenproxyDBRecord, bool) {
	if r == nil || asn == 0 {
		return OpenproxyDBRecord{}, false
	}
	record, ok := r.records[asn]
	return record, ok
}

// Count returns the number of unique ASNs loaded.
func (r *ASNOverlayReader) Count() int {
	if r == nil {
		return 0
	}
	return len(r.records)
}

// Close is a no-op; data is held entirely in memory.
func (r *ASNOverlayReader) Close() error {
	return nil
}
