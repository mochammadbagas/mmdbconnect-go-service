package database

import (
	"strings"
	"testing"

	"github.com/mochammadbagas/mmdbconnect-go-service/internal/config"
)

func TestEnsureDatabaseExists_Validation(t *testing.T) {
	tests := []struct {
		name      string
		dbName    string
		shouldErr bool
	}{
		{"valid standard", "mmdbconnect_go_service", false},
		{"valid alphanumeric", "db123_prod", false},
		{"invalid with backtick", "db`name", true},
		{"invalid with semicolon", "db;DROP TABLE users;--", true},
		{"invalid with space", "db name", true},
		{"invalid with hyphen", "db-name", true},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				DBName: tt.dbName,
				DBHost: "127.0.0.1",
				DBPort: "99999", // Invalid port so network connection fails fast if validation passes
			}
			err := ensureDatabaseExists(cfg)
			if tt.shouldErr {
				if err == nil {
					t.Fatalf("expected error for dbName %q, got nil", tt.dbName)
				}
				expectedSubstr := "invalid database name"
				if !strings.Contains(err.Error(), expectedSubstr) {
					t.Fatalf("expected error containing %q, got %q", expectedSubstr, err.Error())
				}
			}
		})
	}
}
