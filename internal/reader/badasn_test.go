package reader

import (
	"os"
	"testing"
)

func TestOpenBadASNListIncludesManualHostingASNs(t *testing.T) {
	path := writeTempFile(t, "badasn-*.csv", "ASN\n")

	r, err := OpenBadASNList(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, asn := range ManuallyAddedBadASNs {
		if !r.Contains(asn) {
			t.Fatalf("Contains(%d) = false, want true for manually added hosting ASN", asn)
		}
	}
}

func TestOpenBadASNListIncludesManualHostingASNsWithEmptyFile(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "badasn-empty-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := OpenBadASNList(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, asn := range ManuallyAddedBadASNs {
		if !r.Contains(asn) {
			t.Fatalf("Contains(%d) = false, want true for manually added hosting ASN", asn)
		}
	}
}
