/*
 **********************************************************************
 * -------------------------------------------------------------------
 * Project Name : Abdal 4iProto Server
 * File Name    : types.go
 * Programmer   : Ebrahim Shafiei (EbraSha)
 * Email        : Prof.Shafiei@Gmail.com
 * Created On   : 2026-06-30 04:12:29
 * Description  : Configuration and domain type definitions for the SSH tunnel server
 * -------------------------------------------------------------------
 *
 * "Coding is an engaging and beloved hobby for me. I passionately and insatiably pursue knowledge in cybersecurity and programming."
 * – Ebrahim Shafiei
 *
 **********************************************************************
 */

package config

import "strings"

// UserRole defines the user authorization level.
type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
)

// NormalizeRole normalizes a role string to either RoleAdmin or RoleUser.
func NormalizeRole(role UserRole) UserRole {
	if strings.EqualFold(string(role), string(RoleAdmin)) {
		return RoleAdmin
	}
	return RoleUser
}

// TrafficStats holds per-user traffic counters persisted across sessions.
type TrafficStats struct {
	Username           string `json:"username"`
	IP                 string `json:"ip"`
	LastBytesSent      int64  `json:"last_bytes_sent"`
	LastBytesReceived  int64  `json:"last_bytes_received"`
	LastBytesTotal     int64  `json:"last_bytes_total"`
	TotalBytesSent     int64  `json:"total_bytes_sent"`
	TotalBytesReceived int64  `json:"total_bytes_received"`
	TotalBytes         int64  `json:"total_bytes"`
	LastTimestamp      string `json:"last_timestamp"`
}

// User represents a single authenticated account with policy fields.
type User struct {
	Username           string   `json:"username"`
	Password           string   `json:"password"`
	Role               UserRole `json:"role"`
	BlockedDomains     []string `json:"blocked_domains"`
	BlockedIPs         []string `json:"blocked_ips"`
	Log                string   `json:"log"`
	MaxSessions        int      `json:"max_sessions"`
	SessionTTLSeconds  int      `json:"session_ttl_seconds"`
	MaxSpeedKBPS       int      `json:"max_speed_kbps"`
	MaxTotalMB         int      `json:"max_total_mb"`
	EvictOnMaxSessions *bool    `json:"evict_on_max_sessions,omitempty"`
}

// ValidateAndSetDefaults normalizes and sets safe default values for user configuration.
func (u *User) ValidateAndSetDefaults() {
	u.Role = NormalizeRole(u.Role)
	if u.MaxSessions <= 0 {
		u.MaxSessions = DefaultMaxSessionsPerUser
	}
	if u.SessionTTLSeconds <= 0 {
		u.SessionTTLSeconds = DefaultSessionTTLSeconds
	}
	if u.MaxSpeedKBPS < 0 {
		u.MaxSpeedKBPS = 0
	}
	if u.MaxTotalMB < 0 {
		u.MaxTotalMB = 0
	}
}

// ServerConfig is the top-level server configuration loaded from JSON.
type ServerConfig struct {
	Ports                 []int  `json:"ports"`
	Shell                 string `json:"shell"`
	MaxAuthAttempts       int    `json:"max_auth_attempts"`
	ServerVersion         string `json:"server_version"`
	PrivateKeyFile        string `json:"private_key_file"`
	PublicKeyFile         string `json:"public_key_file"`
	DNSTTEnabled          bool   `json:"dnstt_enabled"`
	DNSTTListen           string `json:"dnstt_listen"`
	DNSTTResolver         string `json:"dnstt_resolver"`
	DNSTTNameserver       string `json:"dnstt_nameserver"`
	DNSTTPublicKey        string `json:"dnstt_public_key"`
	DNSTTPSK              string `json:"dnstt_psk"`
	DNSTTMaxSessionsPerIP int    `json:"dnstt_max_sessions_per_ip"`
	DNSTTIdleTimeoutSec   int    `json:"dnstt_idle_timeout_seconds"`
	UDPIdleTimeoutSec     int    `json:"udp_idle_timeout_seconds"`
	UDPSocketBufferKB     int    `json:"udp_socket_buffer_kb"`
	EvictOnMaxSessions    bool   `json:"evict_on_max_sessions"`
}

// ValidateAndSetDefaults validates ports and applies safe defaults to server configuration.
func (c *ServerConfig) ValidateAndSetDefaults() {
	var validPorts []int
	for _, p := range c.Ports {
		if p >= 1 && p <= 65535 {
			validPorts = append(validPorts, p)
		}
	}
	if len(validPorts) == 0 {
		validPorts = []int{2222}
	}
	c.Ports = validPorts

	if c.MaxAuthAttempts <= 0 {
		c.MaxAuthAttempts = 3
	}
	if c.UDPIdleTimeoutSec <= 0 {
		c.UDPIdleTimeoutSec = DefaultUDPIdleTimeoutSec
	}
	if c.UDPSocketBufferKB <= 0 {
		c.UDPSocketBufferKB = 4096
	}
	if c.DNSTTMaxSessionsPerIP <= 0 {
		c.DNSTTMaxSessionsPerIP = 8
	}
	if c.DNSTTIdleTimeoutSec <= 0 {
		c.DNSTTIdleTimeoutSec = 60
	}
	if c.Shell == "" {
		c.Shell = "/bin/bash"
	}
}

// BlockedIPs is the on-disk representation of the global IP block list.
type BlockedIPs struct {
	Blocked []string `json:"blocked"`
}

// UDPSocketBufferBytes returns the effective UDP socket buffer size in bytes.
func (c *ServerConfig) UDPSocketBufferBytes() int {
	if c.UDPSocketBufferKB > 0 {
		return c.UDPSocketBufferKB * 1024
	}
	return DefaultUDPSocketBufferBytes
}

// UDPIdleTimeoutSeconds returns the effective UDP idle timeout in seconds.
func (c *ServerConfig) UDPIdleTimeoutSeconds() int64 {
	if c.UDPIdleTimeoutSec > 0 {
		return int64(c.UDPIdleTimeoutSec)
	}
	return DefaultUDPIdleTimeoutSec
}
