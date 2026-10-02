// Package database provides an explicit Gorm connection entry point.
// Install a driver and supply its dialector when adding persistence.
// The HTTP starter does not call Open or run migrations.
package database

import (
	"errors"
	"gorm.io/gorm"
)

func Open(dialector gorm.Dialector, options ...gorm.Option) (*gorm.DB, error) {
	if dialector == nil {
		return nil, errors.New("database dialector is required")
	}
	return gorm.Open(dialector, options...)
}
