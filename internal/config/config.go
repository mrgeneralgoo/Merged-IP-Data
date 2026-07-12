package config

// Database download URLs
const (
	IPLocationDBReleaseBaseURL = "https://github.com/sapics/ip-location-db/releases/download/latest"

	GeoLite2CityURL        = "https://github.com/P3TERX/GeoLite.mmdb/releases/latest/download/GeoLite2-City.mmdb"
	GeoLite2ASNURL         = "https://github.com/P3TERX/GeoLite.mmdb/releases/latest/download/GeoLite2-ASN.mmdb"
	IPinfoLiteURL          = "https://github.com/NetworkCats/IPinfoLite-Download/releases/latest/download/ipinfo_lite.mmdb"
	DBIPCityIPv4URL        = IPLocationDBReleaseBaseURL + "/dbip-city-ipv4.mmdb"
	DBIPCityIPv6URL        = IPLocationDBReleaseBaseURL + "/dbip-city-ipv6.mmdb"
	RouteViewsASNURL       = IPLocationDBReleaseBaseURL + "/origin-asn.mmdb"
	GeoWhoisCountryURL     = IPLocationDBReleaseBaseURL + "/geolite2-country.mmdb"
	QQWryURL               = "https://github.com/nmgliangwei/qqwry.ipdb/releases/latest/download/qqwry.ipdb"
	OpenproxyDBURL         = "https://github.com/NetworkCats/OpenProxyDB/releases/latest/download/proxy_blocks.csv"
	ICloudPrivateRelayURL  = "https://mask-api.icloud.com/egress-ip-ranges.csv"
	X4BVPNASNURL           = "https://github.com/X4BNet/lists_vpn/raw/refs/heads/main/input/vpn/ASN.txt"
	X4BDatacenterASNURL    = "https://raw.githubusercontent.com/X4BNet/lists_vpn/refs/heads/main/input/datacenter/ASN.txt"
	X4BMullvadVPNURL       = "https://raw.githubusercontent.com/X4BNet/lists_vpn/refs/heads/main/input/vpn/ips/mullvadvpn.txt"
	X4BPIAVPNURL           = "https://raw.githubusercontent.com/X4BNet/lists_vpn/refs/heads/main/input/vpn/ips/pia.txt"
	X4BProtonVPNURL        = "https://raw.githubusercontent.com/X4BNet/lists_vpn/refs/heads/main/input/vpn/ips/protonvpn.txt"
	X4BDatacenterProtonURL = "https://raw.githubusercontent.com/X4BNet/lists_vpn/refs/heads/main/input/datacenter/ips/protonvpn.txt"
	NordVPNIPListURL       = "https://gist.githubusercontent.com/JamoCA/eedaf4f7cce1cb0aeb5c1039af35f0b7/raw/cb6568528820c09e94cac7ef3461bc6cbf792e7e/NordVPN-Server-IP-List.txt"
	BadIPListURL           = "https://github.com/NetworkCats/badiplist/releases/latest/download/badiplist.txt"
	TorRelaysURL           = "https://onionoo.torproject.org/details?type=relay&running=true&fields=or_addresses,exit_addresses"
	AnycastV4URL           = "https://raw.githubusercontent.com/bgptools/anycast-prefixes/refs/heads/master/anycatch-v4-prefixes.txt"
	AnycastV6URL           = "https://raw.githubusercontent.com/bgptools/anycast-prefixes/refs/heads/master/anycatch-v6-prefixes.txt"
	BadASNListURL          = "https://raw.githubusercontent.com/brianhama/bad-asn-list/refs/heads/master/bad-asn-list.csv"
)

// Local file paths for downloaded databases
const (
	GeoLite2CityFile        = "download/GeoLite2-City.mmdb"
	GeoLite2ASNFile         = "download/GeoLite2-ASN.mmdb"
	IPinfoLiteFile          = "download/ipinfo_lite.mmdb"
	DBIPCityIPv4File        = "download/dbip-city-ipv4.mmdb"
	DBIPCityIPv6File        = "download/dbip-city-ipv6.mmdb"
	RouteViewsASNFile       = "download/origin-asn.mmdb"
	GeoWhoisCountryFile     = "download/geolite2-country.mmdb"
	QQWryFile               = "download/qqwry.ipdb"
	OpenproxyDBFile         = "download/proxy_blocks.csv"
	ICloudPrivateRelayFile  = "download/icloud-private-relay.csv"
	X4BVPNASNFile           = "download/x4b-vpn-asn.txt"
	X4BDatacenterASNFile    = "download/x4b-datacenter-asn.txt"
	X4BMullvadVPNFile       = "download/x4b-mullvad-vpn.txt"
	X4BPIAVPNFile           = "download/x4b-pia-vpn.txt"
	X4BProtonVPNFile        = "download/x4b-proton-vpn.txt"
	X4BDatacenterProtonFile = "download/x4b-datacenter-proton-vpn.txt"
	NordVPNIPListFile       = "download/nordvpn-server-ips.txt"
	BadIPListFile           = "download/badiplist.txt"
	TorRelaysFile           = "download/tor_relays.json"
	AnycastV4File           = "download/anycast-v4.txt"
	AnycastV6File           = "download/anycast-v6.txt"
	BadASNListFile          = "download/bad-asn-list.csv"
)

// Output file path
const (
	OutputFile = "Merged-IP.mmdb"
)

// Supported languages for multi-language names
var SupportedLanguages = []string{
	"de",    // German
	"en",    // English
	"es",    // Spanish
	"fr",    // French
	"ja",    // Japanese
	"pt-BR", // Portuguese (Brazil)
	"ru",    // Russian
	"zh-CN", // Chinese (Simplified)
}

// Database metadata
const (
	DatabaseType        = "Merged-IP-City-ASN"
	DatabaseDescription = "Merged IP geolocation database combining GeoLite2, IPinfo Lite, and DB-IP data"
)

// Download settings
const (
	DownloadTimeout     = 300 // seconds
	DownloadMaxRetries  = 3
	DownloadRetryDelay  = 5 // seconds
	DownloadConcurrency = 7
)

// DatabaseSource represents a database source with its URL and local path
type DatabaseSource struct {
	Name string
	URL  string
	Path string
}

// GetAllSources returns all database sources for downloading
func GetAllSources() []DatabaseSource {
	return []DatabaseSource{
		{Name: "GeoLite2-City", URL: GeoLite2CityURL, Path: GeoLite2CityFile},
		{Name: "GeoLite2-ASN", URL: GeoLite2ASNURL, Path: GeoLite2ASNFile},
		{Name: "IPinfo-Lite", URL: IPinfoLiteURL, Path: IPinfoLiteFile},
		{Name: "DB-IP-IPv4", URL: DBIPCityIPv4URL, Path: DBIPCityIPv4File},
		{Name: "DB-IP-IPv6", URL: DBIPCityIPv6URL, Path: DBIPCityIPv6File},
		{Name: "Origin-ASN", URL: RouteViewsASNURL, Path: RouteViewsASNFile},
		{Name: "GeoLite2-Country", URL: GeoWhoisCountryURL, Path: GeoWhoisCountryFile},
		{Name: "QQWry-Chunzhen", URL: QQWryURL, Path: QQWryFile},
		{Name: "OpenProxyDB", URL: OpenproxyDBURL, Path: OpenproxyDBFile},
		{Name: "iCloud-Private-Relay", URL: ICloudPrivateRelayURL, Path: ICloudPrivateRelayFile},
		{Name: "X4B-VPN-ASN", URL: X4BVPNASNURL, Path: X4BVPNASNFile},
		{Name: "X4B-Datacenter-ASN", URL: X4BDatacenterASNURL, Path: X4BDatacenterASNFile},
		{Name: "X4B-Mullvad-VPN", URL: X4BMullvadVPNURL, Path: X4BMullvadVPNFile},
		{Name: "X4B-PIA-VPN", URL: X4BPIAVPNURL, Path: X4BPIAVPNFile},
		{Name: "X4B-Proton-VPN", URL: X4BProtonVPNURL, Path: X4BProtonVPNFile},
		{Name: "X4B-Datacenter-Proton-VPN", URL: X4BDatacenterProtonURL, Path: X4BDatacenterProtonFile},
		{Name: "NordVPN-Server-IPs", URL: NordVPNIPListURL, Path: NordVPNIPListFile},
		{Name: "BadIPList", URL: BadIPListURL, Path: BadIPListFile},
		{Name: "Tor-Relays", URL: TorRelaysURL, Path: TorRelaysFile},
		{Name: "Anycast-V4", URL: AnycastV4URL, Path: AnycastV4File},
		{Name: "Anycast-V6", URL: AnycastV6URL, Path: AnycastV6File},
		{Name: "BadASNList", URL: BadASNListURL, Path: BadASNListFile},
	}
}
