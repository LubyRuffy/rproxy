package checkproxy

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"
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

type ProxyResult struct {
	Valid          bool                   // 是否代理
	Cost           time.Duration          // 耗时
	Error          error                  // 错误信息，如果有
	Header         http.Header            // header返回
	Url            string                 // 完整的代理url
	IP             string                 // 代理请求时对外的ip，不一定跟解析的ip相等
	Port           int                    //端口
	Upstream       string                 // 是否有上一跳的信息
	Geo            map[string]interface{} // geo信息
	SupportConnect bool                   // 是否支持connect，在http的情况下有效
	UrlParsed      *url.URL               // 解析后的结果
	ProxyLevel     ProxyAnonymityLevel
}

func (pr ProxyResult) String() string {
	d, _ := json.Marshal(pr)
	return string(d)
}
