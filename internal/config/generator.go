package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Sathira-Laksahan/vless-linux/internal/parser"
)

// SingBoxConfig represents the top-level Sing-box configuration.
type SingBoxConfig struct {
	Log       LogConfig   `json:"log"`
	DNS       DNSConfig   `json:"dns"`
	Inbounds  []any       `json:"inbounds"`
	Outbounds []any       `json:"outbounds"`
	Route     RouteConfig `json:"route"`
}

type LogConfig struct {
	Level     string `json:"level"`
	Timestamp bool   `json:"timestamp"`
}

type DNSServerConfig struct {
	Tag     string `json:"tag"`
	Address string `json:"address"`
	Detour  string `json:"detour,omitempty"`
}

type DNSRuleConfig struct {
	Outbound []string `json:"outbound,omitempty"`
	Server   string   `json:"server"`
}

type DNSConfig struct {
	Servers  []DNSServerConfig `json:"servers"`
	Rules    []DNSRuleConfig   `json:"rules,omitempty"`
	Strategy string            `json:"strategy"`
}

type InboundConfig struct {
	Type                     string `json:"type"`
	Tag                      string `json:"tag"`
	InterfaceName            string `json:"interface_name"`
	Inet4Address             string `json:"inet4_address"`
	AutoRoute                bool   `json:"auto_route"`
	StrictRoute              bool   `json:"strict_route"`
	Stack                    string `json:"stack"`
	Sniff                    bool   `json:"sniff"`
	SniffOverrideDestination bool   `json:"sniff_override_destination"`
}

type MixedInboundConfig struct {
	Type       string `json:"type"`
	Tag        string `json:"tag"`
	Listen     string `json:"listen"`
	ListenPort int    `json:"listen_port"`
}

type DirectOutbound struct {
	Type string `json:"type"`
	Tag  string `json:"tag"`
}

type DNSOutbound struct {
	Type string `json:"type"`
	Tag  string `json:"tag"`
}

type BlockOutbound struct {
	Type string `json:"type"`
	Tag  string `json:"tag"`
}

type UTLSConfig struct {
	Enabled     bool   `json:"enabled"`
	Fingerprint string `json:"fingerprint"`
}

type TLSConfig struct {
	Enabled    bool        `json:"enabled"`
	ServerName string      `json:"server_name"`
	UTLS       *UTLSConfig `json:"utls,omitempty"`
	Insecure   bool        `json:"insecure,omitempty"`
}

type TransportConfig struct {
	Type    string            `json:"type,omitempty"`
	Path    string            `json:"path,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type VlessOutbound struct {
	Type       string           `json:"type"`
	Tag        string           `json:"tag"`
	Server     string           `json:"server"`
	ServerPort int              `json:"server_port"`
	UUID       string           `json:"uuid"`
	Flow       string           `json:"flow,omitempty"`
	Network    string           `json:"network,omitempty"`
	TLS        *TLSConfig       `json:"tls,omitempty"`
	Transport  *TransportConfig `json:"transport,omitempty"`
}

type RouteRuleConfig struct {
	Protocol    string   `json:"protocol,omitempty"`
	Port        []uint16 `json:"port,omitempty"`
	Network     string   `json:"network,omitempty"`
	IPIsPrivate bool     `json:"ip_is_private,omitempty"`
	Outbound    string   `json:"outbound"`
}

type RouteConfig struct {
	AutoDetectInterface bool              `json:"auto_detect_interface"`
	Rules               []RouteRuleConfig `json:"rules"`
	Final               string            `json:"final"`
}

// GenerateSingBoxConfig creates a Sing-box config tailored for Linux TUN full-system bypass.
func GenerateSingBoxConfig(vless *parser.VlessConfig) (*SingBoxConfig, error) {
	if vless == nil {
		return nil, fmt.Errorf("vless config is nil")
	}

	cfg := &SingBoxConfig{
		Log: LogConfig{
			Level:     "info",
			Timestamp: true,
		},
		DNS: DNSConfig{
			Servers: []DNSServerConfig{
				{
					Tag:     "remote-dns-tcp",
					Address: "tcp://1.1.1.1",
					Detour:  "proxy-out",
				},
				{
					Tag:     "remote-dns-udp",
					Address: "1.1.1.1",
					Detour:  "proxy-out",
				},
				{
					Tag:     "google-dns-tcp",
					Address: "tcp://8.8.8.8",
					Detour:  "proxy-out",
				},
				{
					Tag:     "google-dns-udp",
					Address: "8.8.8.8",
					Detour:  "proxy-out",
				},
				{
					Tag:     "local-dns",
					Address: "local",
					Detour:  "direct",
				},
			},
			Strategy: "ipv4_only",
		},
		Inbounds: []any{
			InboundConfig{
				Type:                     "tun",
				Tag:                      "tun-in",
				InterfaceName:            "tun0",
				Inet4Address:             "172.19.0.1/30",
				AutoRoute:                true,
				StrictRoute:              false,
				Stack:                    "gvisor",
				Sniff:                    true,
				SniffOverrideDestination: false,
			},
			MixedInboundConfig{
				Type:       "mixed",
				Tag:        "mixed-in",
				Listen:     "127.0.0.1",
				ListenPort: 10808,
			},
		},
		Route: RouteConfig{
			AutoDetectInterface: true,
			Rules: []RouteRuleConfig{
				{
					Protocol: "dns",
					Outbound: "dns-out",
				},
				{
					Port:     []uint16{53},
					Outbound: "dns-out",
				},
				{
					Protocol: "quic",
					Outbound: "block",
				},
				{
					IPIsPrivate: true,
					Outbound:    "direct",
				},
			},
			Final: "proxy-out",
		},
	}

	// Prepare VLESS outbound
	vlessOut := VlessOutbound{
		Type:       "vless",
		Tag:        "proxy-out",
		Server:     ResolveHost(vless.Server),
		ServerPort: vless.Port,
		UUID:       vless.UUID,
		Flow:       vless.Flow,
	}

	// TLS & uTLS setup
	if vless.Security == "tls" || vless.Security == "reality" {
		fp := vless.Fingerprint
		if fp == "" {
			fp = "chrome"
		}

		vlessOut.TLS = &TLSConfig{
			Enabled:    true,
			ServerName: vless.SNI,
			Insecure:   vless.Insecure,
			UTLS: &UTLSConfig{
				Enabled:     true,
				Fingerprint: fp,
			},
		}
	}

	// Transport options (WebSocket, gRPC, etc.)
	if vless.Network == "ws" {
		headers := make(map[string]string)
		if vless.Host != "" {
			headers["Host"] = vless.Host
		}
		vlessOut.Transport = &TransportConfig{
			Type:    "ws",
			Path:    vless.Path,
			Headers: headers,
		}
	}

	cfg.Outbounds = []any{
		vlessOut,
		DirectOutbound{
			Type: "direct",
			Tag:  "direct",
		},
		DNSOutbound{
			Type: "dns",
			Tag:  "dns-out",
		},
		BlockOutbound{
			Type: "block",
			Tag:  "block",
		},
	}

	return cfg, nil
}

// SaveConfigToFile serializes the Sing-box config as formatted JSON.
func SaveConfigToFile(cfg *SingBoxConfig, outputPath string) error {
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ResolveHost resolves a hostname to an IP address to avoid bootstrap DNS deadlocks on restricted networks.
func ResolveHost(host string) string {
	if net.ParseIP(host) != nil {
		return host
	}

	// Known default host mappings (fallback for zero-rated bootstrapping)
	var knownHosts = map[string]string{
		"lw03.sahanwickramasinghe.me": "172.255.210.120",
	}

	// 1. Try local system DNS with a quick 2-second timeout
	resolver := &net.Resolver{
		PreferGo: true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if ips, err := resolver.LookupIP(ctx, "ip4", host); err == nil && len(ips) > 0 {
		for _, ip := range ips {
			if ip.To4() != nil {
				return ip.String()
			}
		}
	}

	// 2. Fallback: Cloudflare DoH over HTTPS (port 443 is typically open on bypass packages)
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://1.1.1.1/dns-query?name="+host+"&type=A", nil)
	if err == nil {
		req.Header.Set("Accept", "application/dns-json")
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var result struct {
				Answer []struct {
					Type int    `json:"type"`
					Data string `json:"data"`
				} `json:"Answer"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
				for _, ans := range result.Answer {
					if ans.Type == 1 && net.ParseIP(ans.Data) != nil {
						return ans.Data
					}
				}
			}
		}
	}

	// 3. Fallback: Check known hosts
	if ip, ok := knownHosts[host]; ok {
		return ip
	}

	return host
}
