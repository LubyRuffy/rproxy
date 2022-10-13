package models

import (
	"github.com/glebarez/sqlite"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	gdb *gorm.DB // mysql数据库
)

// SetupDB 配置db
func SetupDB(dsn string) (*gorm.DB, error) {
	var err error

	cfg := &gorm.Config{
		//Logger: logger.Recorder.LogMode(logger.Silent),
	}
	if viper.GetBool("debug.dbsql") {
		cfg.Logger = logger.Default.LogMode(logger.Info)
	}

	gdb, err = gorm.Open(sqlite.Open(dsn), cfg)
	if err != nil {
		return nil, err
	}

	if err = gdb.AutoMigrate(&Proxy{}, &CheckLog{}, &User{}, &UserProxy{}); err != nil {
		return nil, err
	}

	return gdb, nil
}

// GetDB 获取db
func GetDB() *gorm.DB {
	return gdb
}
