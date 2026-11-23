package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/mochammadbagas/mmdbconnect-go-service/internal/config"

	_ "github.com/go-sql-driver/mysql"
)

var validDBNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// Init initializes the MySQL database connection, pool, migrations, and seed data.
func Init(cfg *config.Config, fresh bool) (*sql.DB, error) {
	// Safeguard: Prevent accidental database reset in production
	if fresh && cfg != nil && cfg.AppEnv == "production" {
		return nil, errors.New("cannot run migrations with -fresh flag in production environment to prevent data loss")
	}

	// Auto-create database if not exists (similar to Laravel artisan behavior)
	if err := ensureDatabaseExists(cfg); err != nil {
		slog.Warn("Notice: Automatic database creation check bypassed or failed", slog.String("database", cfg.DBName), slog.String("error", err.Error()))
	}

	// DSN format with UTC timezone, parseTime=true, utf8mb4 charset, and multiStatements=true (required by golang-migrate to run multi-query migration files)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&collation=utf8mb4_0900_ai_ci&parseTime=true&loc=UTC&time_zone='%%2B00:00'&multiStatements=true",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName,
	)

	slog.Info("Connecting to MySQL database...", slog.String("host", cfg.DBHost), slog.String("port", cfg.DBPort), slog.String("database", cfg.DBName))
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Connection Pool configuration
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.DBConnMaxIdleTime)

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping MySQL server: %w", err)
	}

	// Run migrations
	if err := RunMigrations(db, cfg.DBName, fresh); err != nil {
		return nil, fmt.Errorf("failed to run database migrations: %w", err)
	}

	// Seed initial data if empty
	if err := Seed(db, cfg); err != nil {
		return nil, fmt.Errorf("failed to run database seeder: %w", err)
	}

	return db, nil
}

// ensureDatabaseExists connects to the MySQL instance without a database selected
// and creates the target database if it does not already exist (matching Laravel artisan behavior).
func ensureDatabaseExists(cfg *config.Config) error {
	if cfg == nil || cfg.DBName == "" {
		return nil
	}

	if !validDBNamePattern.MatchString(cfg.DBName) {
		return fmt.Errorf("invalid database name '%s': only alphanumeric and underscore characters are allowed", cfg.DBName)
	}

	serverDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&loc=UTC",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort,
	)

	serverDB, err := sql.Open("mysql", serverDSN)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL server: %w", err)
	}
	defer serverDB.Close()

	if err := serverDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping MySQL server root connection: %w", err)
	}

	query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;", cfg.DBName)
	if _, err := serverDB.Exec(query); err != nil {
		return fmt.Errorf("failed to create database '%s': %w", cfg.DBName, err)
	}

	slog.Info("Database schema verified/created successfully", slog.String("database", cfg.DBName))
	return nil
}
