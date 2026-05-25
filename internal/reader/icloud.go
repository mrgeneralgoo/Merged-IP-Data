package reader

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"net/netip"
	"os"
	"strings"
)

// LoadICloudPrivateRelayRanges reads Apple's iCloud Private Relay egress CSV
// and merges each CIDR into the proxy data as both proxy and VPN.
//
// The CSV has no header. The first column is an IPv4 or IPv6 CIDR; remaining
// columns contain location metadata that is not needed for proxy tagging.
func (r *OpenproxyDBReader) LoadICloudPrivateRelayRanges(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("failed to open iCloud Private Relay ranges file: %w", err)
	}
	defer file.Close()

	bufferedReader := bufio.NewReaderSize(file, 256*1024)
	csvReader := csv.NewReader(bufferedReader)
	csvReader.FieldsPerRecord = -1
	csvReader.ReuseRecord = true

	record := OpenproxyDBRecord{
		IsProxy:     true,
		IsVPN:       true,
		IsAnonymous: true,
	}

	count := 0
	lineNum := 0
	for {
		lineNum++
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, fmt.Errorf("failed to read iCloud Private Relay CSV line %d: %w", lineNum, err)
		}
		if len(row) == 0 || strings.TrimSpace(row[0]) == "" {
			continue
		}

		prefix, err := netip.ParsePrefix(strings.TrimSpace(row[0]))
		if err != nil {
			return count, fmt.Errorf("invalid iCloud Private Relay CIDR on line %d: %w", lineNum, err)
		}

		prefix = canonicalPrefix(prefix)
		r.addCIDRRange(prefix, record)
		r.icloudPrivateRelayRanges = append(r.icloudPrivateRelayRanges, prefix)
		count++
	}

	if count > 0 {
		if err := r.rebuildCIDRSet(); err != nil {
			return count, fmt.Errorf("failed to rebuild CIDR lookup set: %w", err)
		}
	}

	return count, nil
}

// ICloudPrivateRelayRanges returns the iCloud Private Relay CIDRs loaded from
// Apple's egress list.
func (r *OpenproxyDBReader) ICloudPrivateRelayRanges() []netip.Prefix {
	return r.icloudPrivateRelayRanges
}
