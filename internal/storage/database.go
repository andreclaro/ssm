package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/andreclaro/ssm/internal/config"
)

// DB represents the database connection
var DB *gorm.DB

// InitDB initializes the database connection and runs migrations
func InitDB() error {
	// Avoid re-initialization if DB is already set
	if DB != nil {
		return nil
	}
	cfg := config.GetConfig()
	if cfg == nil {
		return fmt.Errorf("configuration not initialized")
	}

	// Ensure database directory exists
	dbDir := filepath.Dir(cfg.Database.Path)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Configure GORM logger
	var logLevel logger.LogLevel
	if logrus.GetLevel() == logrus.DebugLevel {
		logLevel = logger.Info
	} else {
		logLevel = logger.Error
	}

	gormLogger := logger.Default
	gormLogger.LogMode(logLevel)

	// Connect to database
	var err error
	DB, err = gorm.Open(sqlite.Open(cfg.Database.Path), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run migrations
	if err := runMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	logrus.WithField("path", cfg.Database.Path).Info("Database initialized")
	return nil
}

// runMigrations runs database migrations
func runMigrations() error {
	// Auto-migrate the schema
	if err := DB.AutoMigrate(&Instance{}, &Tag{}, &Region{}, &Profile{}); err != nil {
		return fmt.Errorf("failed to migrate schema: %w", err)
	}

	// Initialize regions and profiles if this is a fresh database
	regionRepo := NewRegionRepository()
	if err := regionRepo.InitializeRegions(); err != nil {
		return fmt.Errorf("failed to initialize regions: %w", err)
	}

	profileRepo := NewProfileRepository()
	if err := profileRepo.InitializeProfiles(); err != nil {
		return fmt.Errorf("failed to initialize profiles: %w", err)
	}

	logrus.Debug("Database migrations completed")
	return nil
}

// Close closes the database connection
func Close() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
