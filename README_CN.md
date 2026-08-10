# speedtest-x-go

这是 `speedtest-x` 的 Go 重写版，保留原来的页面和接口路径：

- `/index.html` 测速页
- `/results.html` 测速结果
- `/chart.html` 线性图表
- `/backend/garbage.php`
- `/backend/empty.php`
- `/backend/getIP.php`
- `/backend/report.php`
- `/backend/results-api.php`

## Docker

```bash
docker build -t speedtest-x-go .
docker run -d -p 9001:80 -v speedtest-x-go-logs:/speedlogs speedtest-x-go
```

可用环境变量：

- `WEBPORT=80`
- `MAX_LOG_COUNT=100`
- `IP_SERVICE=ip.sb`，也支持 `ipinfo.io`
- `IPINFO_APIKEY=`，使用 `ipinfo.io` 时可选
- `SAME_IP_MULTI_LOGS=false`
- `SPEEDLOGS_DIR=/speedlogs`
- `TIME_ZONE=Asia/Shanghai`

数据保存为 `${SPEEDLOGS_DIR}/speedlogs.json`。
