package merger

import (
	"net"
	"testing"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
)

func TestExactASNInsertionPreservesNarrowerPrimaryNetwork(t *testing.T) {
	tree, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType:            "test",
		IPVersion:               4,
		IncludeReservedNetworks: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	m := Merger{tree: tree}

	_, fallback, _ := net.ParseCIDR("10.0.0.0/8")
	if _, err := m.insertExactASN(fallback, ASNRecord{Number: 64500, Organization: "Fallback"}); err != nil {
		t.Fatal(err)
	}
	_, primary, _ := net.ParseCIDR("10.1.0.0/16")
	if _, err := m.insertExactASN(primary, ASNRecord{Number: 64501, Organization: "Primary"}); err != nil {
		t.Fatal(err)
	}

	assertASNNumber(t, tree, "10.0.0.1", 64500)
	assertASNNumber(t, tree, "10.1.0.1", 64501)
}

func assertASNNumber(t *testing.T, tree *mmdbwriter.Tree, ip string, want mmdbtype.Uint32) {
	t.Helper()
	_, data := tree.Get(net.ParseIP(ip).To4())
	record, ok := data.(mmdbtype.Map)
	if !ok {
		t.Fatalf("record for %s = %T, want map", ip, data)
	}
	asn, ok := record[keyASN].(mmdbtype.Map)
	if !ok || asn[keyASNumber] != want {
		t.Fatalf("ASN for %s = %#v, want %d", ip, record[keyASN], want)
	}
}
