package dao

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"final-exam-savior/backend/internal/config"
)

type DAO struct {
	db    *gorm.DB
	redis *redis.Client
}

func New(ctx context.Context, cfg config.Config) (*DAO, error) {
	db, err := openDB(cfg)
	if err != nil {
		return nil, err
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &DAO{db: db, redis: rdb}, nil
}

func (d *DAO) Gorm() *gorm.DB { return d.db }

func (d *DAO) Redis() *redis.Client { return d.redis }

func (d *DAO) RawDB() (*sql.DB, error) { return d.db.DB() }

func (d *DAO) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.Close(); err != nil {
		return err
	}
	return d.redis.Close()
}

func openDB(cfg config.Config) (*gorm.DB, error) {
	gormCfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)}
	db, err := gorm.Open(mysql.Open(cfg.Database.DSN), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetime) * time.Minute)
	return db, nil
}
