/*
 **********************************************************************
 * -------------------------------------------------------------------
 * Project Name : Abdal 4iProto Server
 * File Name    : loader.go
 * Programmer   : Ebrahim Shafiei (EbraSha)
 * Email        : Prof.Shafiei@Gmail.com
 * Created On   : 2026-06-30 04:12:29
 * Description  : Loads users and server configuration from JSON files
 * -------------------------------------------------------------------
 *
 * "Coding is an engaging and beloved hobby for me. I passionately and insatiably pursue knowledge in cybersecurity and programming."
 * – Ebrahim Shafiei
 *
 **********************************************************************
 */

package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"Abdal_4iProto_Server/core/paths"
)

// RateLimiterInit is called for each user with a speed cap after users are loaded.
type RateLimiterInit func(username string, maxSpeedKBPS int)

// ParseUsersData parses users from either modern JSON array format or legacy map format,
// validating and applying safe defaults.
func ParseUsersData(data []byte) (map[string]User, error) {
	var userList []User
	if err := json.Unmarshal(data, &userList); err == nil {
		u := make(map[string]User, len(userList))
		for _, user := range userList {
			if user.Username == "" {
				continue
			}
			user.ValidateAndSetDefaults()
			u[user.Username] = user
		}
		return u, nil
	}

	var oldUserPass map[string]string
	if err := json.Unmarshal(data, &oldUserPass); err != nil {
		return nil, fmt.Errorf("failed to parse users data in either format: %w", err)
	}

	u := make(map[string]User, len(oldUserPass))
	for username, password := range oldUserPass {
		user := User{
			Username: username,
			Password: password,
			Role:     RoleUser,
		}
		user.ValidateAndSetDefaults()
		u[username] = user
	}
	return u, nil
}

// LoadUsers reads and parses users.json from the executable directory.
func LoadUsers(rateInit RateLimiterInit) {
	exeDir, err := paths.ExecutableDir()
	if err != nil {
		log.Fatalf("Failed to get executable directory: %v", err)
	}

	fullPath := filepath.Join(exeDir, UsersFileName)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		log.Fatalf("Failed to read users file: %v", err)
	}

	u, err := ParseUsersData(data)
	if err != nil {
		log.Fatalf("Failed to parse users file: %v", err)
	}

	for _, user := range u {
		if rateInit != nil && user.MaxSpeedKBPS > 0 {
			rateInit(user.Username, user.MaxSpeedKBPS)
		}
	}

	SetUsers(u)
	log.Printf("✅ Loaded %d users with role-based access control", len(u))
}

// LoadServerConfig reads and parses server_config.json from the executable directory.
func LoadServerConfig() {
	exeDir, err := paths.ExecutableDir()
	if err != nil {
		log.Fatalf("Failed to get executable directory: %v", err)
	}

	fullPath := filepath.Join(exeDir, ServerConfigFileName)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		log.Fatalf("Failed to read server config file: %v", err)
	}

	var cfg ServerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("Failed to parse server config file: %v", err)
	}
	cfg.ValidateAndSetDefaults()
	SetServerConfig(cfg)
}

// PrivateKeyPath resolves the host private key path relative to the executable directory.
func PrivateKeyPath() (string, error) {
	exeDir, err := paths.ExecutableDir()
	if err != nil {
		return "", err
	}
	keyFile := Server().PrivateKeyFile
	if keyFile == "" {
		keyFile = DefaultPrivateKey
	}
	return filepath.Join(exeDir, keyFile), nil
}

// TrafficFilePath returns the per-user traffic stats file path.
func TrafficFilePath(username string) (string, error) {
	exeDir, err := paths.ExecutableDir()
	if err != nil {
		return "", err
	}
	filename := fmt.Sprintf("traffic_%s.json", username)
	return filepath.Join(exeDir, UsersTrafficDir, filename), nil
}
