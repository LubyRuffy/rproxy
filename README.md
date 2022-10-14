# rproxy

自动化的代理服务器。

## 背景
我们平时的工作主要是信息采集，很多平台会有拦截机制，无论是IP对应国家的拦截，还是请求频率的拦截，都会直接影响了效果。

因此需要一套系统，能够每次请求自动切换IP地址。

## Feature
- 提供api接口接收代理服务器的地址，并且验证入库
  - [x] /check 支持
    - [x] http
    - [x] socks5
    - [x] https
    - [x] socks4
    - [x] url/host/ip+port
  - 支持并发限制
  - [x] 失败也要记录日志
  - [x] 支持延迟参数（以服务器所在地为准，因为后续直接把服务器作为请求代理）
  - [x] 支持缓存，一天内不对同一个ip和端口进行多次检查请求
- 代理服务器包含如下一些验证：
  - [x] 是否支持http请求代理
  - [x] 是否支持https请求代理，很多网站都是https网站，就不能用不支持CONNECT的http代理服务器
  - [x] 国家
  - [x] 是否匿名
    - [x] 有无请求源ip的地址
  - [x] 是否高匿名
    - [x] 有无代理相关的header头字段
- [x] 提供api获取代理列表
  - [x] /list 输出对应的代理属性
- [x] 本身提供http[s]/socks5代理功能
  - [ ] 支持账号验证
- [x] 支持账号
  - [x] 支持注册
  - [x] 支持简单的页面
  - [x] 支持验证，通过X-Rproxy-Token头进行 ```curl -H "X-Rproxy-Token: 1234" http://127.0.0.1:8089/api/v1/list | jq```
- [x] 支持过滤器设置，通过X-Rproxy-Filter进行 ```curl -H "X-Rproxy-Filter: type=socks5" http://127.0.0.1:8089 ip.bmh.im/c```
  - [x] 支持转发时删除过滤器
- [x] 支持设置每次测试的proxy个数，通过X-Rproxy-Limit进行 ```curl -x https://127.0.0.1:8088/ --proxy-user 'user:pass' --proxy-insecure https://ip.bmh.im -i --proxy-header "X-Rproxy-Limit: 1" -v```
  - [x] 支持转发时删除limit设置
- [x] 支持数据库存储
  - [x] 支持sqlite
- [x] 支持tls模式https
  - [x] 支持自生成证书，以及加载已经生成的证书
- [x] 支持并发线程池，控制并发数量

## 测试
- [ ] http代理服务器
  - [ ] 支持http
  - [ ] 支持https代理
- [ ] https代理服务器
  - [ ] 支持http
  - [ ] 支持https代理
- 支持socks5代理
- 支持取最快的代理（每次取三条）

## 案例
### 启动服务器
```shell
./rproxy --tls
```

### 添加代理测试
```shell
curl 'https://127.0.0.1:8088/api/v1/check?url=https://1.1.1.1' --user 'user:pass' -k
curl 'https://127.0.0.1:8088/api/v1/check?url=http://2.2.2.2:8080' --user 'user:pass' -k
curl 'https://127.0.0.1:8088/api/v1/check?url=socks5://3.3.3.3:1080' --user 'user:pass' -k
```

### 测试代理
```shell
# 本地代理是http，远端是http
curl -x http://127.0.0.1:8088/ --proxy-user 'user:pass' http://ip.bmh.im -i

# 本地是http，远端是https
curl -x http://127.0.0.1:8088/ --proxy-user 'user:pass' --proxy-insecure https://ip.bmh.im -i

# 本地是https，远端是http
curl -x https://127.0.0.1:8088/ --proxy-user 'user:pass' --proxy-insecure http://ip.bmh.im -i

# 本地是https，远端是https
curl -x https://127.0.0.1:8088/ --proxy-user 'user:pass' --proxy-insecure https://ip.bmh.im -i

# 请求http，后端取一条测试
curl -x https://127.0.0.1:8088/ --proxy-user 'user:pass' --proxy-insecure http://ip.bmh.im -i -H "X-Rproxy-Limit: 1"

# 请求https，后端取一条测试，因为用到了CONNECT，所以要用proxy-header
curl -x https://127.0.0.1:8088/ --proxy-user 'user:pass' --proxy-insecure http://ip.bmh.im -i --proxy-header "X-Rproxy-Limit: 1" -v
```

### 指定一个代理服务器测试
```shell
curl -x https://127.0.0.1:8088/ --proxy-user 'user:pass' --proxy-insecure http://ip.bmh.im -i --proxy-header "X-Rproxy-Filter: ip=1.1.1.1" -v
```

### 循环测试
```shell
while true; do curl -x https://127.0.0.1:8088/ --proxy-insecure --proxy-user "user:pass" ip.bmh.im/g; echo ""; done
```