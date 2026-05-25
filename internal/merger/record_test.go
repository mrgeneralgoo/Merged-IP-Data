package merger

import "testing"

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
