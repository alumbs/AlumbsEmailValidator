FROM golang:1.23-alpine

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY main.go .

RUN go build -o trumail .

EXPOSE 8080
ENV SOURCE_ADDR=""
CMD ["./trumail"]