package reader

import (
	"bufio"
	"fmt"
	"net/netip"
	"os"
	"strings"
)

// LoadVPNProviderCIDRRanges reads one or more plain-text VPN provider prefix
// files and merges each prefix into the proxy data as VPN and hosting.
// Blank lines are skipped, '#' starts a comment, and whitespace can separate
// prefixes on the same line.
func (r *OpenproxyDBReader) LoadVPNProviderCIDRRanges(paths ...string) (int, error) {
	return r.LoadVPNProviderCIDRRangesWithRecord(OpenproxyDBRecord{
		IsVPN:       true,
		IsHosting:   true,
		IsAnonymous: true,
	}, paths...)
}

// LoadVPNProviderCIDRRangesWithRecord reads one or more plain-text provider
// prefix files and merges each prefix into the proxy data with the supplied
// flags.
func (r *OpenproxyDBReader) LoadVPNProviderCIDRRangesWithRecord(record OpenproxyDBRecord, paths ...string) (int, error) {
	record = normalizeOpenproxyRecord(record)

	count := 0
	for _, path := range paths {
		n, err := r.loadVPNProviderCIDRFile(path, record)
		count += n
		if err != nil {
			return count, err
		}
	}

	if count > 0 {
		if err := r.rebuildCIDRSet(); err != nil {
			return count, fmt.Errorf("failed to rebuild CIDR lookup set: %w", err)
		}
	}

	return count, nil
}

func (r *OpenproxyDBReader) loadVPNProviderCIDRFile(path string, record OpenproxyDBRecord) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("failed to open VPN provider CIDR file %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	count := 0
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		for _, token := range strings.Fields(scanner.Text()) {
			if strings.HasPrefix(token, "#") {
				break
			}

			prefix, err := parseCIDROrAddrPrefix(token)
			if err != nil {
				return count, fmt.Errorf("invalid VPN provider CIDR in %s on line %d: %w", path, lineNum, err)
			}

			prefix = canonicalPrefix(prefix)
			r.addCIDRRangeWithSource(prefix, record, cidrRangeSourceVPNProvider)
			r.vpnProviderRanges = append(r.vpnProviderRanges, CIDRRange{
				Prefix: prefix,
				Record: record,
			})
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("error reading VPN provider CIDR file %s: %w", path, err)
	}

	return count, nil
}

func parseCIDROrAddrPrefix(token string) (netip.Prefix, error) {
	prefix, err := netip.ParsePrefix(token)
	if err == nil {
		return prefix, nil
	}

	addr, addrErr := netip.ParseAddr(token)
	if addrErr != nil {
		return netip.Prefix{}, err
	}
	addr = addr.Unmap()
	bits := 32
	if addr.Is6() {
		bits = 128
	}
	return netip.PrefixFrom(addr, bits), nil
}

// VPNProviderRanges returns the loaded third-party VPN provider CIDRs.
func (r *OpenproxyDBReader) VPNProviderRanges() []CIDRRange {
	return append([]CIDRRange(nil), r.vpnProviderRanges...)
}
