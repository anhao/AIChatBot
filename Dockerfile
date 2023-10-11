FROM golang:1.18-alpine AS builder
MAINTAINER Alone88
WORKDIR /app
COPY . .

ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct


RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o wechat_bot main.go

FROM scratch

WORKDIR /app

COPY --from=builder /app/wechat_bot .

EXPOSE 8080

CMD ["./wechat_bot"]