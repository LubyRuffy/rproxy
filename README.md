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
  - 支持缓存，一天内不对同一个ip和端口进行多次检查请求
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
- [x] 支持数据库存储
  - [x] 支持sqlite

## 测试
- http代理服务器
- 支持http/https代理
- 支持socks5代理
- 支持取最快的代理（每次取三条）