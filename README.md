# speedtest-x-go

Go rewrite of speedtest-x with the original UI and backend URL paths preserved.

## Container Image

Every push builds and publishes the image to GitHub Container Registry. The default branch also publishes `latest`:

```text
ghcr.io/bymao/speedtest-x-go:latest
```

Version tags are published when you push tags like `v1.0.0`.

## Docker Deployment

```bash
docker run -d \
  --name speedtest-x-go \
  --restart unless-stopped \
  -p 9001:80 \
  -v speedtest-x-go-logs:/speedlogs \
  ghcr.io/bymao/speedtest-x-go:latest
```

Open:

```text
http://SERVER_IP:9001/index.html
http://SERVER_IP:9001/results.html
http://SERVER_IP:9001/chart.html
```

Local build:

```bash
docker build -t speedtest-x-go .
docker run -d -p 9001:80 -v speedtest-x-go-logs:/speedlogs speedtest-x-go
```

## Docker Compose Deployment

Use the included `docker-compose.yml`:

```bash
docker compose up -d
```

Or create one manually:

```yaml
services:
  speedtest-x-go:
    image: ghcr.io/bymao/speedtest-x-go:latest
    container_name: speedtest-x-go
    restart: unless-stopped
    ports:
      - "9001:80"
    environment:
      WEBPORT: "80"
      MAX_LOG_COUNT: "100"
      IP_SERVICE: "ip.sb"
      SAME_IP_MULTI_LOGS: "false"
      TIME_ZONE: "Asia/Shanghai"
    volumes:
      - speedtest-x-go-logs:/speedlogs

volumes:
  speedtest-x-go-logs:
```

Environment variables:

- `WEBPORT=80`
- `MAX_LOG_COUNT=100`
- `IP_SERVICE=ip.sb` or `ipinfo.io`
- `IPINFO_APIKEY=`
- `SAME_IP_MULTI_LOGS=false`
- `SPEEDLOGS_DIR=/speedlogs`
- `TIME_ZONE=Asia/Shanghai`

Data is stored in `${SPEEDLOGS_DIR}/speedlogs.json`.
