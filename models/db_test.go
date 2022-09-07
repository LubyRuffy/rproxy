package models

import (
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

func TestGetDB(t *testing.T) {
	type User struct {
		gorm.Model
		Name string
	}

	type Proxy struct {
		gorm.Model
		Name string
	}

	// UserProxy 用户对应代理表
	type UserProxy struct {
		gorm.Model
		UserID int  `gorm:"primaryKey;autoIncrement:false;uniqueIndex:idx_user_proxy,priority:1"`
		User   User `gorm:"foreignKey:UserID"`

		ProxyID int   `gorm:"primaryKey;autoIncrement:false;uniqueIndex:idx_user_proxy,priority:2"`
		Proxy   Proxy `gorm:"foreignKey:ProxyID"`
	}

	// 确保文件生成，并且有表结构
	var err error
	dbfile := filepath.Join(os.TempDir(), time.Now().Format("20060102150405.sqlite"))
	defer os.Remove(dbfile)
	db, err := gorm.Open(sqlite.Open(dbfile), &gorm.Config{Logger: logger.Default.LogMode(logger.Info)})
	assert.Nil(t, err)
	assert.NotNil(t, db)
	err = db.AutoMigrate(&User{}, &Proxy{}, &UserProxy{})
	assert.Nil(t, err)

	err = db.Save(&UserProxy{
		User: User{
			Name: "a",
		},
		Proxy: Proxy{
			Name: "b",
		},
	}).Error
	assert.Nil(t, err)
	var ab1 UserProxy
	err = db.Preload("User").Preload("Proxy").First(&ab1).Error
	assert.Nil(t, err)
	assert.Equal(t, ab1.User.Name, "a")
	assert.Equal(t, ab1.Proxy.Name, "b")
}
