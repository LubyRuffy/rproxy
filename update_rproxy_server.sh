#!/usr/bin/bash
# update
go install github.com/LubyRuffy/rproxy@latest
# restart
sudo pkill rproxy | sudo nohup sh -c "`go env GOPATH`/bin/rproxy -addr :8088" 1>rproxy_out.txt 2>rproxy_err.txt &
