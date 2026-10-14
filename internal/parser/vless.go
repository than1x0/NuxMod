package parser

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// VlessConfig holds the parsed VLESS URI configuration.
type VlessConfig struct {
	Remarks     string `json:"remarks"`
	UUID        string `json:"uuid"`
	Server      string `json:"server"`
	Port        int    `json:"port"`
	Network     string `json:"network"`     // tcp, ws, grpc, etc.
	Security    string `json:"security"`    // none, tls, reality
	SNI         string `json:"sni"`         // Server Name Indication
	Fingerprint string `json:"fingerprint"` // chrome, firefox, etc.
	Flow        string `json:"flow"`        // xtls-rprx-vision, etc.
	Path        string `json:"path"`        // WebSocket or gRPC path
	Host        string `json:"host"`        // WebSocket Host header
	PublicKey   string `json:"public_key"`  // Reality public key
	ShortID     string `json:"short_id"`    // Reality short id
	ALPN        string `json:"alpn"`        // h2,http/1.1
	Insecure    bool   `json:"insecure"`    // Skip certificate verification (required for SNI spoofing)
}

// ParseVlessLink parses a standard vless:// URI string into a VlessConfig struct.
func ParseVlessLink(rawURI string) (*VlessConfig, error) {
	rawURI = strings.TrimSpace(rawURI)
	if !strings.HasPrefix(rawURI, "vless://") {
		return nil, errors.New("invalid protocol: link must start with 'vless://'")
	}

	u, err := url.Parse(rawURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URI: %w", err)
	}

	cfg := &VlessConfig{
		Network:     "tcp",
		Security:    "none",
		Fingerprint: "chrome",
	}

	// Extract remarks / tag from Fragment
	if u.Fragment != "" {
		remarks, err := url.QueryUnescape(u.Fragment)
		if err == nil {
			cfg.Remarks = remarks
		} else {
			cfg.Remarks = u.Fragment
		}
	} else {
		cfg.Remarks = "VLESS-Node"
	}

	// Extract UUID from User info
	if u.User == nil || u.User.Username() == "" {
		return nil, errors.New("missing UUID in VLESS link")
	}
	cfg.UUID = u.User.Username()

	// Extract Server Host and Port
	hostStr := u.Hostname()
	if hostStr == "" {
		return nil, errors.New("missing server host in VLESS link")
	}
	cfg.Server = hostStr

	portStr := u.Port()
	if portStr == "" {
		cfg.Port = 443 // default HTTPS port for VLESS TLS
	} else {
		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 || port > 65535 {
			return nil, fmt.Errorf("invalid port '%s'", portStr)
		}
		cfg.Port = port
	}

	// Parse Query Parameters
	query := u.Query()

	if netType := query.Get("type"); netType != "" {
		cfg.Network = strings.ToLower(netType)
	}

	if sec := query.Get("security"); sec != "" {
		cfg.Security = strings.ToLower(sec)
	}

	if sni := query.Get("sni"); sni != "" {
		cfg.SNI = sni
	} else {
		// Fallback to server address if SNI not specified
		cfg.SNI = cfg.Server
	}

	if fp := query.Get("fp"); fp != "" {
		cfg.Fingerprint = strings.ToLower(fp)
	}

	if flow := query.Get("flow"); flow != "" {
		cfg.Flow = flow
	}

	if path := query.Get("path"); path != "" {
		cfg.Path = path
	}

	if host := query.Get("host"); host != "" {
		cfg.Host = host
	}

	if pbk := query.Get("pbk"); pbk != "" {
		cfg.PublicKey = pbk
	}

	if sid := query.Get("sid"); sid != "" {
		cfg.ShortID = sid
	}

	if alpn := query.Get("alpn"); alpn != "" {
		cfg.ALPN = alpn
	}

	// In SNI spoofing, the SNI belongs to a spoofed domain (e.g. aka.ms, zoom.us)
	// while the certificate belongs to the proxy server (e.g. lw03.sahanwickramasinghe.me).
	// Therefore, certificate hostname validation must be skipped (Insecure = true).
	insecureVal := query.Get("allowInsecure")
	if insecureVal == "" {
		insecureVal = query.Get("insecure")
	}
	if insecureVal == "1" || strings.ToLower(insecureVal) == "true" || (cfg.SNI != "" && cfg.SNI != cfg.Server) {
		cfg.Insecure = true
	}

	return cfg, nil
}
