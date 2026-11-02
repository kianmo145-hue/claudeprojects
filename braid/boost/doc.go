// Package boost lets a phone share its mobile data with a computer on the same Wi-Fi, so the
// computer can merge that data with its own connections ("booster" mode).
//
// The phone runs a Server on its Wi-Fi address and announces itself with UDP broadcasts. The
// computer listens for the announcements, pairs once with a six-digit code shown on both screens,
// and then opens one tunnel per TCP flow. Each tunnel is authenticated with a key that both sides
// derived during pairing; the key itself never crosses the network.
//
// Wire format, after the first byte of a TCP connection:
//
//	'J'  pairing: newline-delimited JSON messages (see pair.go)
//	'T'  tunnel:  server sends a 16-byte challenge, client sends one JSON line with the target and
//	     an HMAC, server answers one JSON line, then raw bytes flow both ways (see tunnel.go)
//
// The package uses only the standard library so it can be shared by the Windows app and the
// Android library (gomobile).
package boost

// Default ports. The announcement port is UDP, the service port TCP.
const (
	AnnouncePort = 47800
	ServicePort  = 47801
	Service      = "braid"
	Version      = 1
)
