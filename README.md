# speedtest-x-go

Go rewrite of speedtest-x with the original UI and backend URL paths preserved.

## Docker

```bash
docker build -t speedtest-x-go .
docker run -d -p 9001:80 -v speedtest-x-go-logs:/speedlogs speedtest-x-go
```

Environment variables:

- `WEBPORT=80`
- `MAX_LOG_COUNT=100`
- `IP_SERVICE=ip.sb` or `ipinfo.io`
- `IPINFO_APIKEY=`
- `SAME_IP_MULTI_LOGS=false`
- `SPEEDLOGS_DIR=/speedlogs`
- `TIME_ZONE=Asia/Shanghai`
