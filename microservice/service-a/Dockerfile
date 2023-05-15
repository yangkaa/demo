# 设置基础镜像为golang:alpin
FROM registry.cn-hangzhou.aliyuncs.com/yangkaa/golang:alpine

WORKDIR /app

COPY . .

ENV GOPROXY=https://goproxy.cn

RUN go build -o service-a .

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/service-a .

ENTRYPOINT ["./service-a"]
