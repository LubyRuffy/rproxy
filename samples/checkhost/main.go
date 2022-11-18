package main

import (
	"flag"
	"fmt"
	"github.com/LubyRuffy/rproxy/checkproxy"
)

func main() {
	proxy := flag.String("proxy", "", "")
	flag.Parse()

	if *proxy == "" {
		panic("no proxy to check")
	}
	checkproxy.GetPublicIP()
	fmt.Println(checkproxy.CheckHost(*proxy, nil))
}
