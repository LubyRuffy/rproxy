package models

import (
	"database/sql"
	"github.com/jinzhu/gorm"
)

// ProxyAnonymityLevel 匿名级别
// - 在使用了代理IP访问目标端之后，如果被访问端知道了来访者使用了代理IP，并且识别出来访者的具体IP，那么这就是透明代理；
// - 如果被访问端知道了来访者使用了代理IP，但是无法识别出来访者的具体IP，那么这就是普匿代理；
// - 如果被访问端无法识别出来访者是否使用了代理IP，并且无法识别出来访者的具体IP，那么这就是高匿代理。
// 参考 https://docs.proxymesh.com/article/78-proxy-anonymity-levels
type ProxyAnonymityLevel int

const (
	ProxyAnonymityUnknown     ProxyAnonymityLevel = iota // 未知
	ProxyAnonymityElite                                  // 高匿名，没有 X-Forwarded-For Via From X-Real-IP等header
	ProxyAnonymityAnonymous                              // 普匿代理
	ProxyAnonymityTransparent                            // 透明代理
)

// Proxy 代理表
// sqlite 不支持comment语法，所以不支持gorm:"comment:aaa"
type Proxy struct {
	gorm.Model
	IP              string              `json:"ip"`                                    //ip地址
	Port            int                 `json:"port"`                                  //端口号
	ProxyType       string              `json:"proxy_type"`                            //代理类型http/https/socks5/socks4
	ProxyURL        string              `json:"proxy_url" gorm:"index:idx_url,unique"` //完整代理地址https://p.abc.com:1234
	Country         string              `json:"country" gorm:"index:idx_url"`          //国家，二位码
	Http            bool                `json:"http"`                                  //http代理可访问
	Connect         bool                `json:"https"`                                 //https代理可访问
	ProxyLevel      ProxyAnonymityLevel `json:"proxy_level"`                           //匿名级别
	Latency         int                 `json:"latency"`                               //延迟，单位为ms
	SuccessCount    int                 `json:"success_count"`                         //成功次数
	FailedCount     int                 `json:"failed_count"`                          //失败次数
	LastSuccessTime sql.NullTime        `json:"last_success_time"`                     //最后成功时间
	LastFailedTime  sql.NullTime        `json:"last_failed_time"`                      //最后失败时间
}
