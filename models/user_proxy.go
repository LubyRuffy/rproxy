package models

import "github.com/jinzhu/gorm"

// UserProxy 用户对应代理表
type UserProxy struct {
	gorm.Model
	UserID uint `gorm:"primaryKey;autoIncrement:false;uniqueIndex:idx_user_proxy,priority:1"`
	User   User `gorm:"foreignKey:UserID"`

	ProxyID uint  `gorm:"primaryKey;autoIncrement:false;uniqueIndex:idx_user_proxy,priority:2"`
	Proxy   Proxy `gorm:"foreignKey:ProxyID"`
}
