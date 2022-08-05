package models

import (
	"github.com/jinzhu/gorm"
)

// User 用户表
type User struct {
	gorm.Model
	Email string `json:"email"`
	Token string `json:"token"`
}
