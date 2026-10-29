/*
 **********************************************************************
 * -------------------------------------------------------------------
 * Project Name : Abdal 4iProto Server
 * File Name    : access.go
 * Programmer   : Ebrahim Shafiei (EbraSha)
 * Email        : Prof.Shafiei@Gmail.com
 * Created On   : 2026-06-30 04:12:29
 * Description  : Per-user domain and IP access restrictions with wildcard matching
 * -------------------------------------------------------------------
 *
 * "Coding is an engaging and beloved hobby for me. I passionately and insatiably pursue knowledge in cybersecurity and programming."
 * – Ebrahim Shafiei
 *
 **********************************************************************
 */

package security

import (
	"path/filepath"
	"strings"

	"Abdal_4iProto_Server/core/config"
)

// IsDomainOrIPBlocked checks whether a target is blocked for the given user.
func IsDomainOrIPBlocked(username, target string) bool {
	user, exists := config.GetUser(username)
	if !exists {
		return false
	}

	for _, blockedDomain := range user.BlockedDomains {
		if matchesWildcard(target, blockedDomain) {
			return true
		}
	}

	for _, blockedIP := range user.BlockedIPs {
		if matchesWildcard(target, blockedIP) {
			return true
		}
	}

	return false
}

func matchesWildcard(str, pattern string) bool {
	str = strings.ToLower(strings.TrimSpace(str))
	pattern = strings.ToLower(strings.TrimSpace(pattern))

	if str == pattern {
		return true
	}

	// *.example.com matches example.com directly
	if strings.HasPrefix(pattern, "*.") && str == pattern[2:] {
		return true
	}

	if matched, err := filepath.Match(pattern, str); err == nil && matched {
		return true
	}

	// Substring wildcard: *part*
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") && len(pattern) > 2 {
		return strings.Contains(str, pattern[1:len(pattern)-1])
	}
	// Prefix wildcard: *suffix
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(str, pattern[1:])
	}
	// Suffix wildcard: prefix*
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(str, pattern[:len(pattern)-1])
	}

	return false
}
