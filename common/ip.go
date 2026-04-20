package common

import (
	"net"
	"sync"
)

func IsIP(s string) bool {
	ip := net.ParseIP(s)
	return ip != nil
}

func ParseIP(s string) net.IP {
	return net.ParseIP(s)
}

func IsPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	private := []net.IPNet{
		{IP: net.IPv4(10, 0, 0, 0), Mask: net.CIDRMask(8, 32)},
		{IP: net.IPv4(172, 16, 0, 0), Mask: net.CIDRMask(12, 32)},
		{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(16, 32)},
	}

	for _, privateNet := range private {
		if privateNet.Contains(ip) {
			return true
		}
	}
	return false
}

func IsIpInCIDRList(ip net.IP, cidrList []string) bool {
	for _, cidr := range cidrList {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			if whitelistIP := net.ParseIP(cidr); whitelistIP != nil {
				if ip.Equal(whitelistIP) {
					return true
				}
			}
			continue
		}

		if network.Contains(ip) {
			return true
		}
	}
	return false
}

type parsedIPEntry struct {
	network *net.IPNet
	ip      net.IP
}

var parsedCIDRCache sync.Map // string (comma-joined) -> []parsedIPEntry

func ParseAndCacheCIDRList(cidrList []string) []parsedIPEntry {
	if len(cidrList) == 0 {
		return nil
	}
	key := ""
	for i, c := range cidrList {
		if i > 0 {
			key += ","
		}
		key += c
	}
	if cached, ok := parsedCIDRCache.Load(key); ok {
		return cached.([]parsedIPEntry)
	}
	entries := make([]parsedIPEntry, 0, len(cidrList))
	for _, cidr := range cidrList {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			if ip := net.ParseIP(cidr); ip != nil {
				entries = append(entries, parsedIPEntry{ip: ip})
			}
			continue
		}
		entries = append(entries, parsedIPEntry{network: network})
	}
	parsedCIDRCache.Store(key, entries)
	return entries
}

func IsIpInParsedCIDRList(ip net.IP, entries []parsedIPEntry) bool {
	for _, entry := range entries {
		if entry.network != nil {
			if entry.network.Contains(ip) {
				return true
			}
		} else if entry.ip != nil {
			if ip.Equal(entry.ip) {
				return true
			}
		}
	}
	return false
}
