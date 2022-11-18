package checkproxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
)

var (
	once       sync.Once
	myPublicIP string // 公网ip，用于检查代理是否匿名
)

// GetPublicIP 获取公网IP列表
func GetPublicIP() string {
	once.Do(func() {
		resp, err := http.Get("https://stat.ripe.net/data/whats-my-ip/data.json")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not get IPs: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		var r struct {
			Data struct {
				IP string `json:"ip"`
			} `json:"data"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&r); err != nil {
			fmt.Fprintf(os.Stderr, "Could not get IPs: %v\n", err)
			os.Exit(1)
		}
		myPublicIP = r.Data.IP
	})

	return myPublicIP
}

func ContainsPublicIP(str string) bool {
	return str == myPublicIP || strings.Contains(str, myPublicIP)
}
