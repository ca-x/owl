FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /build/frontend
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY frontend/ .
RUN pnpm build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /build/backend
RUN apk add --no-cache build-base git
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG GIT_COMMIT=unknown
COPY backend/go.mod backend/go.sum ./
RUN GOSUMDB=off GOPROXY=https://goproxy.cn,direct go mod download
COPY backend/ .
COPY --from=frontend /build/backend/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} GOSUMDB=off GOPROXY=https://goproxy.cn,direct go build -trimpath \
    -ldflags="-s -w \
      -X 'owl/backend/pkg/version.Version=${VERSION}' \
      -X 'owl/backend/pkg/version.BuildTime=${BUILD_TIME}' \
      -X 'owl/backend/pkg/version.GitCommit=${GIT_COMMIT}'" \
    -o /out/owl-server ./cmd/server

FROM alpine:3.22
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=build /out/owl-server /usr/local/bin/owl-server
RUN mkdir -p /app/data/uploads
ENV OWL_PORT=8080
EXPOSE 8080
CMD ["/usr/local/bin/owl-server"]
