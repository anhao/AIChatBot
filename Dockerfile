FROM golang:1.18-alpine AS builder
MAINTAINER Alone88
WORKDIR /app
COPY . .

ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct


RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o aichat_bot main.go

FROM alpine

WORKDIR /app

COPY --from=builder /app/aichat_bot .

ENV GIN_MODE=release
ENV MP_APPID=""
ENV MP_SECRET=""
ENV DEFAULT_API_URL="https://api.aigc2d.com/v1"
ENV DEFAULT_API_KEY=""
ENV STREAM=false
ENV ENABLE_HISTORY=false
ENV ENABLE_SEARCH=false
ENV SERPER_KEY=""

EXPOSE 8080

CMD ["./aichat_bot"]