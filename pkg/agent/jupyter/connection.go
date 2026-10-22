// Package jupyter implements the bounded Jupyter wire protocol used between
// Runme and locally managed kernels.
package jupyter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

const maxConnectionFileBytes = 64 * 1024

// Channel identifies one of the five sockets in a Jupyter connection file.
type Channel string

const (
	ChannelShell     Channel = "shell"
	ChannelIOPub     Channel = "iopub"
	ChannelStdin     Channel = "stdin"
	ChannelControl   Channel = "control"
	ChannelHeartbeat Channel = "heartbeat"
)

// ConnectionInfo is the validated subset of a Jupyter connection file needed
// by the direct kernel bridge. Direct connections are intentionally restricted
// to loopback TCP endpoints.
type ConnectionInfo struct {
	Transport       string `json:"transport"`
	IP              string `json:"ip"`
	ShellPort       int    `json:"shell_port"`
	IOPubPort       int    `json:"iopub_port"`
	StdinPort       int    `json:"stdin_port"`
	ControlPort     int    `json:"control_port"`
	HeartbeatPort   int    `json:"hb_port"`
	Key             string `json:"key"`
	SignatureScheme string `json:"signature_scheme"`
}

// LoadConnectionFile reads and validates a bounded Jupyter connection file.
func LoadConnectionFile(path string) (ConnectionInfo, error) {
	if strings.TrimSpace(path) == "" {
		return ConnectionInfo{}, errors.New("connection file path is required")
	}

	f, err := os.Open(path)
	if err != nil {
		return ConnectionInfo{}, fmt.Errorf("open connection file: %w", err)
	}
	defer f.Close()

	raw, err := io.ReadAll(io.LimitReader(f, maxConnectionFileBytes+1))
	if err != nil {
		return ConnectionInfo{}, fmt.Errorf("read connection file: %w", err)
	}
	return ParseConnectionInfo(raw)
}

// ParseConnectionInfo parses and validates a Jupyter connection file payload.
func ParseConnectionInfo(raw []byte) (ConnectionInfo, error) {
	if len(raw) == 0 {
		return ConnectionInfo{}, errors.New("connection file is empty")
	}
	if len(raw) > maxConnectionFileBytes {
		return ConnectionInfo{}, fmt.Errorf("connection file exceeds %d bytes", maxConnectionFileBytes)
	}

	var info ConnectionInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return ConnectionInfo{}, fmt.Errorf("parse connection file: %w", err)
	}
	if err := info.Validate(); err != nil {
		return ConnectionInfo{}, err
	}
	return info, nil
}

// Validate checks that the connection data is complete and safe for a local
// managed kernel.
func (c ConnectionInfo) Validate() error {
	if c.Transport != "tcp" {
		return fmt.Errorf("unsupported Jupyter transport %q", c.Transport)
	}

	ip := net.ParseIP(strings.TrimSpace(c.IP))
	if ip == nil {
		return fmt.Errorf("invalid Jupyter IP %q", c.IP)
	}
	if !ip.IsLoopback() {
		return fmt.Errorf("jupyter IP %q is not loopback", c.IP)
	}
	if c.SignatureScheme != SignatureSchemeHMACSHA256 {
		return fmt.Errorf("unsupported Jupyter signature scheme %q", c.SignatureScheme)
	}

	ports := []struct {
		name string
		port int
	}{
		{"shell_port", c.ShellPort},
		{"iopub_port", c.IOPubPort},
		{"stdin_port", c.StdinPort},
		{"control_port", c.ControlPort},
		{"hb_port", c.HeartbeatPort},
	}
	seen := make(map[int]string, len(ports))
	for _, item := range ports {
		if item.port < 1 || item.port > 65535 {
			return fmt.Errorf("%s must be between 1 and 65535", item.name)
		}
		if previous, ok := seen[item.port]; ok {
			return fmt.Errorf("%s duplicates %s", item.name, previous)
		}
		seen[item.port] = item.name
	}
	return nil
}

// Endpoint returns the tcp:// endpoint for a connection-file channel.
func (c ConnectionInfo) Endpoint(channel Channel) (string, error) {
	var port int
	switch channel {
	case ChannelShell:
		port = c.ShellPort
	case ChannelIOPub:
		port = c.IOPubPort
	case ChannelStdin:
		port = c.StdinPort
	case ChannelControl:
		port = c.ControlPort
	case ChannelHeartbeat:
		port = c.HeartbeatPort
	default:
		return "", fmt.Errorf("unknown Jupyter channel %q", channel)
	}
	if err := c.Validate(); err != nil {
		return "", err
	}
	return "tcp://" + net.JoinHostPort(c.IP, strconv.Itoa(port)), nil
}
