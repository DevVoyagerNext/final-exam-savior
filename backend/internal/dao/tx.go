package dao

import (
	"context"

	"gorm.io/gorm"
)

func (d *DAO) RunInTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return d.db.WithContext(ctx).Transaction(fn)
}

func (d *DAO) Transaction(fn func(tx *gorm.DB) error) error {
	return d.db.Transaction(fn)
}
