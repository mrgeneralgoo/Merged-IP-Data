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
