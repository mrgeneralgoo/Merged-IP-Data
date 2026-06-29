package reader

import "testing"

func TestOpenASNOverlayListsParsesASNsAndInlineComments(t *testing.T) {
	vpnPath := writeTempFile(t, "asn-vpn-*.txt", "AS9009 # M247\n20448\nnot-an-asn\n")
	hostingPath := writeTempFile(t, "asn-hosting-*.txt", "AS20448 # duplicate with hosting\nAS45090 # Tencent cloud\n")

	r, err := OpenASNOverlayLists(
		ASNOverlaySource{
			Path:   vpnPath,
			Record: OpenproxyDBRecord{IsVPN: true},
		},
		ASNOverlaySource{
			Path:   hostingPath,
			Record: OpenproxyDBRecord{IsHosting: true},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if got := r.Count(); got != 3 {
		t.Fatalf("Count() = %d, want 3 unique ASNs", got)
	}

	record, ok := r.Lookup(9009)
	if !ok {
		t.Fatal("Lookup(9009) = false, want true")
	}
	if !record.IsVPN || record.IsHosting || !record.IsAnonymous {
		t.Fatalf("Lookup(9009) = %+v, want VPN and anonymous only", record)
	}

	record, ok = r.Lookup(20448)
	if !ok {
		t.Fatal("Lookup(20448) = false, want true")
	}
	if !record.IsVPN || !record.IsHosting || !record.IsAnonymous {
		t.Fatalf("Lookup(20448) = %+v, want VPN, hosting, and anonymous", record)
	}

	record, ok = r.Lookup(45090)
	if !ok {
		t.Fatal("Lookup(45090) = false, want true")
	}
	if record.IsVPN || !record.IsHosting || record.IsAnonymous {
		t.Fatalf("Lookup(45090) = %+v, want hosting only", record)
	}
}
