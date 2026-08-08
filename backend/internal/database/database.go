// Package database bootstraps the GORM connection to PostgreSQL.
// All queries in the codebase go through GORM — no raw SQL is used.
package database

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Connect opens the PostgreSQL connection through GORM and configures the
// connection pool.
func Connect(dsn string, logLevel string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLoggerFor(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

func gormLoggerFor(level string) gormlogger.Interface {
	switch level {
	case "debug":
		return gormlogger.Default.LogMode(gormlogger.Info)
	case "warn":
		return gormlogger.Default.LogMode(gormlogger.Warn)
	default:
		return gormlogger.Default.LogMode(gormlogger.Error)
	}
}
