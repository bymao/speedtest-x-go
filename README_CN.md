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

## 镜像

每次 push 到 GitHub 后，GitHub Actions 会自动构建并发布 Docker 镜像到 GitHub Container Registry。默认分支会额外发布 `latest`：

```text
ghcr.io/bymao/speedtest-x-go:latest
```

打版本标签时也会发布对应版本镜像，例如：

```bash
git tag v1.0.0
git push origin v1.0.0
```

生成的镜像：

```text
ghcr.io/bymao/speedtest-x-go:v1.0.0
```

如果拉取私有仓库镜像，需要先登录 GHCR：

```bash
echo YOUR_GITHUB_TOKEN | docker login ghcr.io -u bymao --password-stdin
```

公开仓库镜像通常可以直接拉取。

## Docker 部署

直接使用 GitHub 自动构建的镜像：

```bash
docker run -d \
  --name speedtest-x-go \
  --restart unless-stopped \
  -p 9001:80 \
  -v speedtest-x-go-logs:/speedlogs \
  ghcr.io/bymao/speedtest-x-go:latest
```

访问：

```text
http://服务器IP:9001/index.html
http://服务器IP:9001/results.html
http://服务器IP:9001/chart.html
```

本地手动构建：

```bash
docker build -t speedtest-x-go .
docker run -d -p 9001:80 -v speedtest-x-go-logs:/speedlogs speedtest-x-go
```

## Docker Compose 部署

使用仓库里的 `docker-compose.yml`：

```bash
docker compose up -d
```

或手动创建：

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

可用环境变量：

- `WEBPORT=80`
- `MAX_LOG_COUNT=100`
- `IP_SERVICE=ip.sb`，也支持 `ipinfo.io`
- `IPINFO_APIKEY=`，使用 `ipinfo.io` 时可选
- `SAME_IP_MULTI_LOGS=false`
- `SPEEDLOGS_DIR=/speedlogs`
- `TIME_ZONE=Asia/Shanghai`

数据保存为 `${SPEEDLOGS_DIR}/speedlogs.json`。

## GitHub Actions

工作流文件位于 `.github/workflows/docker-image.yml`：

- push 到任意分支：构建并发布分支 tag、`sha-xxxxxxx`
- push 到默认分支：额外发布 `latest`
- push tag `v*`：构建并发布版本 tag
- pull request：只构建验证，不推送镜像
