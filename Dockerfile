FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/speedtest-x-go .

FROM scratch

WORKDIR /app
COPY --from=build /out/speedtest-x-go /speedtest-x-go
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY index.html results.html chart.html speedtest.js speedtest_worker.js favicon.ico ./
COPY chartjs ./chartjs

ENV WEBPORT=80
ENV MAX_LOG_COUNT=100
ENV IP_SERVICE=ip.sb
ENV SAME_IP_MULTI_LOGS=false
ENV SPEEDLOGS_DIR=/speedlogs
ENV TIME_ZONE=Asia/Shanghai

VOLUME ["/speedlogs"]
EXPOSE 80

ENTRYPOINT ["/speedtest-x-go"]
