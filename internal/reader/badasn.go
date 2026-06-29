package reader

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ManuallyAddedBadASNs are ASNs treated as bad/hosting beyond those in the
// upstream bad-asn-list. These entries supplement upstream coverage for ASNs
// that should be treated as hosting by the proxy-detection fallback.
var ManuallyAddedBadASNs = []uint32{
	174,
	906,
	132110,
	32519,
	3214,
	8888,
	6233,
	3258,
	9312,
	4785,
	328383,
	215871,
	949,
	205758,
	35537,
	41767,
	35536,
	57969,
	39220,
	208982,
	50967,
	43992,
	50131,
	49581,
	59711,
	24785,
	53356,
	39704,
	394177,
	12200,
	202954,
	26548,
	60800,
	29452,
	133950,
	51765,
	200000,
	22611,
	204538,
	208636,
	48282,
	3842,
	62097,
	45187,
	55720,
	13830,
	14244,
	63997,
	54641,
	64200,
	55293,
	201983,
	328631,
	212576,
	136052,
	12488,
	40966,
	45152,
	328032,
	24459,
	328621,
	131316,
	36791,
	37352,
	209341,
	36454,
	204800,
	41634,
	48823,
	19133,
	44051,
	41637,
	23747,
	212660,
	33070,
	396073,
	14576,
	12417,
	395092,
	27357,
	202015,
	52465,
	200719,
	36231,
	139686,
	139345,
	12574,
	209220,
	201525,
	40281,
	208332,
	15919,
	19994,
	33993,
	42442,
	29290,
	213166,
	204928,
	61046,
	202786,
	39150,
	140941,
	16003,
	398395,
	395723,
	36218,
	43362,
	7040,
	202882,
	200549,
	196745,
	26338,
	198944,
	29262,
	23881,
	44476,
	198968,
	49485,
	207605,
	265839,
	200979,
	39576,
	197439,
	48040,
	397964,
	200342,
	62752,
	31659,
	200746,
	47549,
	205631,
	59851,
	269048,
	199997,
	61978,
	201064,
	24633,
	31333,
	14567,
	30943,
	200147,
	57286,
	135682,
	213354,
	49493,
	266886,
	139580,
	26481,
	26930,
	136171,
	29302,
	263735,
	42457,
	34499,
	208895,
	54839,
	204164,
	39556,
	198347,
	47823,
	44716,
	12645,
	53914,
	51099,
	42635,
	51294,
	13209,
	49834,
	27640,
	206331,
	52335,
	34420,
	50921,
	213200,
	10747,
	200450,
	200525,
	29713,
	42347,
	47289,
	24558,
	205106,
	16556,
	135822,
	41541,
	202364,
	134926,
	213183,
	45426,
	262603,
	17881,
	14397,
	263700,
	58922,
	45481,
	32647,
	40100,
	11235,
	14160,
	212922,
	40539,
	42508,
	54636,
	33208,
	31981,
	11230,
	210294,
	62838,
	20248,
	18120,
	131214,
	212363,
	33439,
	55229,
	27223,
	18635,
	40374,
	265527,
	13909,
	29883,
	393841,
	267841,
}

// BadASNReader holds the set of ASNs flagged as bad/hosting. IPs whose ASN
// lookup resolves to an entry in this set are treated as hosting/proxy when
// OpenProxyDB does not already mark them as a proxy.
type BadASNReader struct {
	asns map[uint32]struct{}
}

// OpenBadASNList opens and parses the bad-asn-list CSV file at path. The file
// is expected to have a header row identifying an "ASN" column; if no such
// header exists the first column is used. Additional ASNs from
// ManuallyAddedBadASNs are merged into the set regardless of what the file
// contains.
func OpenBadASNList(path string) (*BadASNReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open bad ASN list: %w", err)
	}
	defer file.Close()

	r := &BadASNReader{
		asns: make(map[uint32]struct{}),
	}

	buffered := bufio.NewReaderSize(file, 64*1024)
	csvReader := csv.NewReader(buffered)
	csvReader.FieldsPerRecord = -1
	csvReader.Comment = '#'
	csvReader.ReuseRecord = true

	header, err := csvReader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			r.addManual()
			return r, nil
		}
		return nil, fmt.Errorf("failed to read bad ASN list header: %w", err)
	}

	asnCol := findASNColumn(header)

	// If the first row doesn't look like a header (first field is numeric),
	// treat it as a data row.
	if !looksLikeHeader(header) && asnCol < len(header) {
		if asn, ok := parseASNField(header[asnCol]); ok {
			r.asns[asn] = struct{}{}
		}
	}

	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Skip malformed lines but keep parsing.
			var parseErr *csv.ParseError
			if errors.As(err, &parseErr) {
				continue
			}
			return nil, fmt.Errorf("failed to read bad ASN list: %w", err)
		}
		if asnCol >= len(row) {
			continue
		}
		if asn, ok := parseASNField(row[asnCol]); ok {
			r.asns[asn] = struct{}{}
		}
	}

	r.addManual()
	return r, nil
}

func (r *BadASNReader) addManual() {
	for _, asn := range ManuallyAddedBadASNs {
		r.asns[asn] = struct{}{}
	}
}

// looksLikeHeader returns true when the row is likely a header — i.e. the
// first field isn't parseable as an ASN integer.
func looksLikeHeader(row []string) bool {
	if len(row) == 0 {
		return false
	}
	_, ok := parseASNField(row[0])
	return !ok
}

// findASNColumn returns the index of the column named "asn" (case-insensitive)
// in a header row; if no such column is found it returns 0 (first column).
func findASNColumn(header []string) int {
	for i, col := range header {
		if strings.EqualFold(strings.TrimSpace(col), "asn") {
			return i
		}
	}
	return 0
}

// parseASNField parses a raw CSV field into an ASN number, stripping an
// optional "AS" prefix and surrounding whitespace.
func parseASNField(s string) (uint32, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if len(s) >= 2 {
		prefix := strings.ToUpper(s[:2])
		if prefix == "AS" {
			s = s[2:]
		}
	}
	asn, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, false
	}
	if asn == 0 {
		return 0, false
	}
	return uint32(asn), true
}

// Contains reports whether the given ASN is present in the bad ASN set.
// Returns false for a nil receiver or an unknown ASN (0).
func (r *BadASNReader) Contains(asn uint32) bool {
	if r == nil || asn == 0 {
		return false
	}
	_, ok := r.asns[asn]
	return ok
}

// Count returns the number of bad ASNs loaded, including manually added
// entries. Returns 0 for a nil receiver.
func (r *BadASNReader) Count() int {
	if r == nil {
		return 0
	}
	return len(r.asns)
}

// Close is a no-op; data is held entirely in memory.
func (r *BadASNReader) Close() error {
	return nil
}
