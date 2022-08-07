package models

import (
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSetupDB(t *testing.T) {
	viper.SetDefault("debug.dbsql", true)

	// 确保文件生成，并且有表结构
	var err error
	dbfile := filepath.Join(os.TempDir(), time.Now().Format("20060102150405.sqlite"))
	defer os.Remove(dbfile)
	db, err := SetupDB(dbfile)
	assert.Nil(t, err)
	assert.NotNil(t, db)
	stmt := &gorm.Statement{DB: db}
	stmt.Parse(&Proxy{})
	assert.Equal(t, "proxies", stmt.Schema.Table)
	assert.True(t, GetDB().Migrator().HasTable(stmt.Schema.Table))
	assert.True(t, GetDB().Migrator().HasTable(&Proxy{}))

	// 插入数据
	var count int64
	err = GetDB().Model(&Proxy{}).Count(&count).Error
	assert.Nil(t, err)
	assert.Equal(t, int64(0), count)
	GetDB().Save(&Proxy{
		IP:           "127.0.0.1",
		Port:         8080,
		ProxyType:    "http",
		ProxyURL:     "http://127.0.0.1:8080",
		Http:         true,
		Connect:      false,
		ProxyLevel:   ProxyAnonymityTransparent,
		SuccessCount: 0,
		FailedCount:  0,
	})
	err = GetDB().Model(&Proxy{}).Count(&count).Error
	assert.Nil(t, err)
	assert.Equal(t, int64(1), count)

	// 再次打开表，会不会多次创建表结构？因为HasTable并没有实现
	// 果然，会提示panic: SQL logic error: table "proxies" already exists (1) [recovered]
	// 必须用"gorm.io/driver/sqlite"才可以
	d, err := GetDB().DB()
	assert.Nil(t, err)
	d.Close()
	db, err = SetupDB(dbfile)
	assert.Nil(t, err)
	assert.NotNil(t, db)
}
