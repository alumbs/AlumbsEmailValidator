FROM golang:1.21-alpine

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY main.go .

RUN go mod init trumail-validator && \
    go get github.com/labstack/echo/v4 && \
    go get github.com/labstack/echo/v4/middleware && \
    go build -o trumail .

EXPOSE 8080
ENV SOURCE_ADDR=""
CMD ["./trumail"]