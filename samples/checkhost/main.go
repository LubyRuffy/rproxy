package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/LubyRuffy/rproxy/checkproxy"
	"log"
	"os"
	"sync"
)

/*
fofa search -f host,ip -s 10000 --format=json 'is_domain=true && title="ERROR: The requested URL could not be retrieved" && cert.is_valid=true' | \
jq -s -r 'group_by(.ip)| map({ ip: (.[0].ip), host: (.[0].host) }) | .[] |.host' > host.txt

上面语句功能：
- 先利用gofofa的客户端，提取所有有正确证书的对应真实域名的可能是代理的host列表；
- 再利用jq根据ip去重，只提取一条host进行请求验证
*/
func main() {
	proxy := flag.String("proxy", "", "")
	file := flag.String("file", "", "")
	flag.Parse()

	if *proxy == "" && *file == "" {
		panic("no proxy to check")
	}
	checkproxy.GetPublicIP()

	if *proxy != "" {
		fmt.Println(checkproxy.CheckHost(*proxy, nil))
		return
	}

	if *file != "" {
		file, err := os.Open(*file)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		queueCh := make(chan string, 10)
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for host := range queueCh {
					r := checkproxy.CheckHost(host, nil)
					if r.Valid {
						os.Stderr.WriteString("\n")
						fmt.Println(r)
					} else {
						os.Stderr.WriteString(".")
					}
				}
			}()
		}
		for scanner.Scan() {
			queueCh <- scanner.Text()
		}
		close(queueCh)
		wg.Wait()

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
	}
}
