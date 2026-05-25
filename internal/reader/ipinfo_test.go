package reader

import "testing"

func TestIPinfoASNParsingIsCaseInsensitiveAndCached(t *testing.T) {
	record := IPinfoLiteRecord{ASN: " as64500 "}

	if !record.HasASN() {
		t.Fatal("HasASN() = false, want true for lowercase AS prefix")
	}
	if got := record.GetASNumber(); got != 64500 {
		t.Fatalf("GetASNumber() = %d, want 64500", got)
	}
}

func TestIPinfoHasASNIgnoresInvalidASN(t *testing.T) {
	record := IPinfoLiteRecord{ASN: "ASnot-a-number"}

	if record.HasASN() {
		t.Fatal("HasASN() = true, want false for invalid ASN")
	}
	if got := record.GetASNumber(); got != 0 {
		t.Fatalf("GetASNumber() = %d, want 0", got)
	}
}
