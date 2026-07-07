package common

import (
	"encoding/binary"
	"math/bits"
	"net/netip"
	"strconv"
)

func IPFormatToInt(ip string) (uint32, uint16) {
	var addr netip.Addr
	var port uint16

	addrport, err := netip.ParseAddrPort(ip)
	if err != nil {
		addr, _ = netip.ParseAddr(ip)
	} else {
		addr = addrport.Addr()
		port = addrport.Port()
	}

	return binary.BigEndian.Uint32(addr.AsSlice()), port
}

func IPFormatNoPortToInt(ip string) uint32 {
	intIP, _ := IPFormatToInt(ip)

	return intIP
}

func IPFormatToString(ip string) (string, string) {
	intIP, intPort := IPFormatToInt(ip)

	return strconv.Itoa(int(int32((intIP)))), strconv.Itoa(int(intPort))
}

func IPFormatToStringLE(ip string) (string, string) {
	intIP, intPort := IPFormatToInt(ip)

	// Convert to little endian and print as big endian int
	return strconv.Itoa(int(int32(bits.ReverseBytes32(intIP)))), strconv.Itoa(int(intPort))
}

func IPFormatBytes(ip string) []byte {
	var addr netip.Addr

	addrport, err := netip.ParseAddrPort(ip)
	if err != nil {
		addr, _ = netip.ParseAddr(ip)
	} else {
		addr = addrport.Addr()
	}

	return addr.AsSlice()
}

var (
	reservedIPList = []struct {
		ip   uint32
		mask uint32
	}{
		{IPFormatNoPortToInt("0.0.0.0"), 8},       // RFC1122 "This host on this network"
		{IPFormatNoPortToInt("10.0.0.0"), 8},      // RFC1918 Private-Use
		{IPFormatNoPortToInt("100.64.0.0"), 10},   // RFC6598 Shared Address Space
		{IPFormatNoPortToInt("127.0.0.0"), 8},     // RFC1122 Loopback
		{IPFormatNoPortToInt("169.254.0.0"), 16},  // RFC3927 Link-Local
		{IPFormatNoPortToInt("172.16.0.0"), 12},   // RFC1918 Private-Use
		{IPFormatNoPortToInt("192.0.0.0"), 24},    // RFC6890 IETF Protocol Assignments
		{IPFormatNoPortToInt("192.0.2.0"), 24},    // RFC5737 Documentation (TEST-NET-1)
		{IPFormatNoPortToInt("192.31.196.0"), 24}, // RFC7535 AS112-v4
		{IPFormatNoPortToInt("192.52.193.0"), 24}, // RFC7450 AMT
		{IPFormatNoPortToInt("192.88.99.0"), 24},  // RFC7526 6to4 Relay Anycast
		{IPFormatNoPortToInt("192.168.0.0"), 16},  // RFC1918 Private-Use
		{IPFormatNoPortToInt("192.175.48.0"), 24}, // RFC7534 Direct Delegation AS112 Service
		{IPFormatNoPortToInt("198.18.0.0"), 15},   // RFC2544 Benchmarking
		{IPFormatNoPortToInt("198.51.100.0"), 24}, // RFC5737 Documentation (TEST-NET-2)
		{IPFormatNoPortToInt("203.0.113.0"), 24},  // RFC5737 Documentation (TEST-NET-3)
		{IPFormatNoPortToInt("224.0.0.0"), 4},     // RFC1112 Multicast
		{IPFormatNoPortToInt("240.0.0.0"), 4},     // RFC1112 Reserved for Future Use + RFC919 Limited Broadcast
	}
)

// TODO: Test this
func IsReservedIP(ip uint32) bool {
	for _, reserved := range reservedIPList {
		rMask := 32 - reserved.mask
		if ip>>rMask == reserved.ip>>rMask {
			return true
		}
	}

	return false
}
