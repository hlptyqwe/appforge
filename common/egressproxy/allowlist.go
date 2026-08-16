// Package egressproxy implements AppForge's outbound HTTPS CONNECT proxy.
// It deliberately refuses plaintext forwarding and only connects to targets
// listed by the deployment administrator.
package egressproxy

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

const maximumRules = 1024

type rule struct {
	host   string
	port   string
	suffix bool
}

// Allowlist is an immutable collection of exact host:port and
// *.domain.example:port rules.
type Allowlist struct {
	rules []rule
}

// ParseAllowlist parses one target per line. Blank lines and lines beginning
// with # are ignored. Wildcards are limited to the left-most label.
func ParseAllowlist(reader io.Reader) (*Allowlist, error) {
	if reader == nil {
		return nil, errors.New("egress proxy allowlist is required")
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024), 4096)
	result := &Allowlist{}
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "#", 2)[0])
		if line == "" {
			continue
		}
		if len(result.rules) >= maximumRules {
			return nil, fmt.Errorf("egress proxy allowlist exceeds %d rules", maximumRules)
		}
		parsed, err := parseRule(line)
		if err != nil {
			return nil, fmt.Errorf("egress proxy allowlist line %d: %w", lineNumber, err)
		}
		result.rules = append(result.rules, parsed)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read egress proxy allowlist: %w", err)
	}
	if len(result.rules) == 0 {
		return nil, errors.New("egress proxy allowlist has no targets")
	}
	return result, nil
}

func parseRule(value string) (rule, error) {
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		return rule{}, errors.New("target must be host:port (IPv6 literals require brackets)")
	}
	if parsedPort, err := strconv.Atoi(port); err != nil || parsedPort < 1 || parsedPort > 65535 {
		return rule{}, errors.New("target port is invalid")
	}
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "" || strings.ContainsAny(host, " /?#@") {
		return rule{}, errors.New("target host is invalid")
	}
	parsed := rule{host: host, port: port}
	if strings.HasPrefix(host, "*.") {
		parsed.suffix = true
		parsed.host = strings.TrimPrefix(host, "*.")
		if net.ParseIP(parsed.host) != nil || !validDNSName(parsed.host) {
			return rule{}, errors.New("wildcard target domain is invalid")
		}
	} else if net.ParseIP(host) == nil && !validDNSName(host) {
		return rule{}, errors.New("target domain is invalid")
	}
	return parsed, nil
}

func validDNSName(value string) bool {
	if len(value) == 0 || len(value) > 253 {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
}

// Allows reports whether the normalized CONNECT destination is approved.
func (allowlist *Allowlist) Allows(host, port string) bool {
	if allowlist == nil {
		return false
	}
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	for _, item := range allowlist.rules {
		if item.port != port {
			continue
		}
		if (!item.suffix && host == item.host) ||
			(item.suffix && host != item.host && strings.HasSuffix(host, "."+item.host)) {
			return true
		}
	}
	return false
}
