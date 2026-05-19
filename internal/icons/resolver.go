// Package icons maps device roles to SVG icons embedded in the binary.
// All functions return values suitable for direct use in D2 scripts.
// To add a new icon: drop the .svg into svgs/ and add a line to roleIcon.
// To change a mapping: edit roleIcon. Both require a rebuild.
package icons

import (
	"embed"
	"encoding/base64"
	"strings"
)

//go:embed svgs
var svgFS embed.FS

// roleIcon maps device role strings to SVG filenames within svgs/.
var roleIcon = map[string]string{
	// Switching
	"spine":               "svgs/layer3-switch.svg",
	"leaf":                "svgs/layer2-switch.svg",
	"access_switch":       "svgs/layer2-switch.svg",
	"distribution_switch": "svgs/layer3-switch.svg",

	// Routing & security
	"border_router": "svgs/router.svg",
	"edge_router":   "svgs/router.svg",
	"router":        "svgs/router.svg",
	"firewall":      "svgs/firewall.svg",
	"load_balancer": "svgs/load-balancer.svg",

	// Wireless
	"wireless_ap": "svgs/wireless-ap.svg",

	// Compute
	"server":     "svgs/server.svg",
	"hypervisor": "svgs/hypervisor.svg",
	"storage":    "svgs/storage.svg",
	"laptop":     "svgs/laptop.svg",

	// Sites & WAN constructs
	"abstract_site": "svgs/cloud-generic.svg",
	"datacenter":    "svgs/datacenter.svg",
	"branch_office": "svgs/branch-office.svg",
	"internet":      "svgs/internet.svg",
	"wan_cloud":     "svgs/wan-cloud.svg",
}

const defaultIcon = "svgs/layer2-switch.svg"

// DataURI returns a base64-encoded SVG data URI for a device role.
// Use as the value of the D2 `icon` property on a `shape: image` node.
func DataURI(role string) string {
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(svgFor(role))
}

// DrawioSVGURI returns a URL-encoded SVG data URI safe for draw.io style strings.
// Uses data:image/svg+xml, (no ;base64) to avoid the semicolon in the MIME type
// breaking draw.io's semicolon-delimited style parser. Also substitutes a concrete
// stroke colour for currentColor, which has no CSS parent to inherit from when the
// SVG is rendered as a standalone image.
func DrawioSVGURI(role string) string {
	s := string(svgFor(role))
	s = strings.ReplaceAll(s, "currentColor", "#455A64")
	s = strings.ReplaceAll(s, `"`, "'")   // single quotes are valid SVG; avoids encoding
	s = strings.ReplaceAll(s, "%", "%25") // must be first
	s = strings.ReplaceAll(s, "#", "%23")
	s = strings.ReplaceAll(s, "<", "%3C")
	s = strings.ReplaceAll(s, ">", "%3E")
	return "data:image/svg+xml," + s
}

func svgFor(role string) []byte {
	path, ok := roleIcon[role]
	if !ok {
		path = defaultIcon
	}
	data, err := svgFS.ReadFile(path)
	if err != nil {
		data, _ = svgFS.ReadFile(defaultIcon)
	}
	return data
}
