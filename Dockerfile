FROM golang:1.18-alpine AS builder
MAINTAINER Alone88
WORKDIR /app
COPY . .

ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct


RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o aichat_bot main.go

FROM scratch

WORKDIR /app

COPY --from=builder /app/aichat_bot .

ENV GIN_MODE=release

EXPOSE 8080

CMD ["./aichat_bot"]