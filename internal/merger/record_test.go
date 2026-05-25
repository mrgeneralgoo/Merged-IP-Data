package merger

import (
	"net/netip"
	"testing"
)

func TestWithNameClonesInput(t *testing.T) {
	original := map[string]string{"en": "Beijing"}
	updated := withName(original, "zh-CN", "Beijing CN")

	if _, ok := original["zh-CN"]; ok {
		t.Fatal("withName mutated the input map")
	}
	if updated["en"] != "Beijing" || updated["zh-CN"] != "Beijing CN" {
		t.Fatalf("updated = %#v, want original and added names", updated)
	}
}

func TestApplySchoolASNMatchMarksUniversityOrganization(t *testing.T) {
	record := MergedRecord{
		ASN: ASNRecord{
			Organization: "Example State University",
		},
	}

	applySchoolASNMatch(&record)

	if !record.Proxy.IsSchool {
		t.Fatalf("proxy = %+v, want school", record.Proxy)
	}
}

func TestApplySchoolASNMatchMarksSchoolOrganizationCaseInsensitive(t *testing.T) {
	record := MergedRecord{
		ASN: ASNRecord{
			Organization: "EXAMPLE SCHOOL DISTRICT",
		},
	}

	applySchoolASNMatch(&record)

	if !record.Proxy.IsSchool {
		t.Fatalf("proxy = %+v, want school", record.Proxy)
	}
}

func TestApplySchoolASNMatchIgnoresOtherOrganizations(t *testing.T) {
	record := MergedRecord{
		ASN: ASNRecord{
			Organization: "Example Hosting LLC",
		},
	}

	applySchoolASNMatch(&record)

	if record.Proxy.IsSchool {
		t.Fatalf("proxy = %+v, want not school", record.Proxy)
	}
}

func TestNetipPrefixToIPNetHandlesIPv4AndIPv6(t *testing.T) {
	tests := []struct {
		name   string
		prefix netip.Prefix
		want   string
	}{
		{
			name:   "ipv4",
			prefix: netip.MustParsePrefix("192.0.2.0/24"),
			want:   "192.0.2.0/24",
		},
		{
			name:   "ipv4 mapped",
			prefix: netip.MustParsePrefix("::ffff:192.0.2.0/120"),
			want:   "192.0.2.0/24",
		},
		{
			name:   "ipv6",
			prefix: netip.MustParsePrefix("2001:db8:abcd::/48"),
			want:   "2001:db8:abcd::/48",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network := netipPrefixToIPNet(tt.prefix)
			if network == nil {
				t.Fatal("network is nil")
			}
			if got := network.String(); got != tt.want {
				t.Fatalf("network = %s, want %s", got, tt.want)
			}
		})
	}
}
