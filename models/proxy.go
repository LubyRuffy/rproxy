package models

import (
	"database/sql"
	"github.com/LubyRuffy/rproxy/checkproxy"
	"github.com/jinzhu/gorm"
)

// Proxy 代理表
// sqlite 不支持comment语法，所以不支持gorm:"comment:aaa"
type Proxy struct {
	gorm.Model
	IP              string                         `json:"ip"`                                    //ip地址
	OutIP           string                         `json:"out_ip"`                                //出口ip地址
	Port            int                            `json:"port"`                                  //端口号
	ProxyType       string                         `json:"proxy_type"`                            //代理类型http/https/socks5/socks4
	ProxyURL        string                         `json:"proxy_url" gorm:"index:idx_url,unique"` //完整代理地址https://p.abc.com:1234
	Country         string                         `json:"country"`                               //国家，二位码
	Http            bool                           `json:"http"`                                  //http代理可访问
	Connect         bool                           `json:"https"`                                 //https代理可访问
	IPv6            bool                           `json:"ipv6"`                                  //是否支持ipv6
	ProxyLevel      checkproxy.ProxyAnonymityLevel `json:"proxy_level"`                           //匿名级别
	Latency         int64                          `json:"latency"`                               //延迟，单位为ms
	SuccessCount    int                            `json:"success_count"`                         //成功次数
	FailedCount     int                            `json:"failed_count"`                          //失败次数
	LastSuccessTime sql.NullTime                   `json:"last_success_time"`                     //最后成功时间
	LastFailedTime  sql.NullTime                   `json:"last_failed_time"`                      //最后失败时间
	LastError       string                         `json:"last_error"`                            //最后失败时间
}
