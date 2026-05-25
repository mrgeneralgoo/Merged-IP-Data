package merger

import "strings"

func applySchoolASNMatch(record *MergedRecord) {
	if organizationNameIndicatesSchool(record.ASN.Organization) {
		record.Proxy.IsSchool = true
	}
}

func organizationNameIndicatesSchool(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "university") || strings.Contains(name, "school")
}
