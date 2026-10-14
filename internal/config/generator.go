package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sathira-Laksahan/vless-linux/internal/parser"
)

// SingBoxConfig represents the top-level Sing-box configuration.
type SingBoxConfig struct {
	Log       LogConfig       `json:"log"`
	DNS       DNSConfig       `json:"dns"`
	Inbounds  []InboundConfig `json:"inbounds"`
	Outbounds []any           `json:"outbounds"`
	Route     RouteConfig     `json:"route"`
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

type DirectOutbound struct {
	Type string `json:"type"`
	Tag  string `json:"tag"`
}

type DNSOutbound struct {
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
					Tag:     "remote-dns",
					Address: "tls://1.1.1.1",
					Detour:  "proxy-out",
				},
				{
					Tag:     "remote-dns-backup",
					Address: "tls://8.8.8.8",
					Detour:  "proxy-out",
				},
				{
					Tag:     "local-dns",
					Address: "local",
					Detour:  "direct",
				},
			},
			Strategy: "prefer_ipv4",
		},
		Inbounds: []InboundConfig{
			{
				Type:                     "tun",
				Tag:                      "tun-in",
				InterfaceName:            "tun0",
				Inet4Address:             "172.19.0.1/30",
				AutoRoute:                true,
				StrictRoute:              true,
				Stack:                    "gvisor",
				Sniff:                    true,
				SniffOverrideDestination: false,
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
		Server:     vless.Server,
		ServerPort: vless.Port,
		UUID:       vless.UUID,
		Flow:       vless.Flow,
		Network:    vless.Network,
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
