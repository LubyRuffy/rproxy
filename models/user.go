package models

import (
	"github.com/jinzhu/gorm"
)

// User 用户表
type User struct {
	gorm.Model
	Email string `json:"email" gorm:"uniqueIndex:idx_user,priority:1"`
	Token string `json:"token" gorm:"uniqueIndex:idx_user,priority:2"`
}
