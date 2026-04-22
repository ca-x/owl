FROM golang:1.26-alpine AS build
WORKDIR /app
RUN apk add --no-cache build-base git
COPY go.mod go.sum ./
RUN GOSUMDB=off GOPROXY=https://goproxy.cn,direct go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=arm64 GOSUMDB=off GOPROXY=https://goproxy.cn,direct go build -o /out/owl-server ./cmd/server

FROM alpine:3.22
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=build /out/owl-server /usr/local/bin/owl-server
RUN mkdir -p /app/data/uploads
ENV OWL_PORT=8080
EXPOSE 8080
CMD ["/usr/local/bin/owl-server"]
