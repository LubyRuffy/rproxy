package models

import (
	"github.com/jinzhu/gorm"
)

// CheckLog 失败的日志记录表
type CheckLog struct {
	gorm.Model
	ProxyType string `json:"proxy_type"` //代理类型http/https/socks5/socks4
	Host      string `json:"host"`       //代理host:p.abc.com:1234
	Error     string `json:"error"`      //代理host:p.abc.com:1234
}
