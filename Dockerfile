FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod ./
COPY go.su[m] ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o skwad .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /build/skwad .
COPY static[/] ./static/
EXPOSE 8080
CMD ["./skwad"]
