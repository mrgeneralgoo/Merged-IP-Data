package reader

import "testing"

func TestGeoLiteCoordinatesDistinguishMissingFromZero(t *testing.T) {
	var record GeoLite2CityRecord
	record.Location.TimeZone = "Etc/UTC"
	if !record.HasLocationData() {
		t.Fatal("timezone should count as location data")
	}
	if !record.HasGeoData() {
		t.Fatal("timezone-only GeoLite record should count as geo data")
	}
	if record.HasPrimaryGeoData() {
		t.Fatal("timezone-only GeoLite record should not count as primary geo data")
	}
	if record.HasCoordinates() {
		t.Fatal("missing coordinates should not be reported as present")
	}

	zero := 0.0
	record.Location.Latitude = &zero
	record.Location.Longitude = &zero
	lat, lon, ok := record.Coordinates()
	if !ok || lat != 0 || lon != 0 {
		t.Fatalf("Coordinates() = %v, %v, %v; want 0, 0, true", lat, lon, ok)
	}
}

func TestDBIPCoordinatesDistinguishMissingFromZero(t *testing.T) {
	var record DBIPCityRecord
	record.Timezone = "Etc/UTC"
	if !record.HasLocationData() {
		t.Fatal("timezone should count as location data")
	}
	if !record.HasGeoData() {
		t.Fatal("timezone-only DB-IP record should count as geo data")
	}
	if record.HasCoordinates() {
		t.Fatal("missing coordinates should not be reported as present")
	}

	zero := float32(0)
	record.Latitude = &zero
	record.Longitude = &zero
	lat, lon, ok := record.Coordinates()
	if !ok || lat != 0 || lon != 0 {
		t.Fatalf("Coordinates() = %v, %v, %v; want 0, 0, true", lat, lon, ok)
	}
}
