FROM oven/bun:latest AS builder

WORKDIR /build
COPY web/package.json .


# ⚠️ 移除 bun.lockb，确保不会缓存旧版本 rollup
RUN rm -f bun.lockb && bun install


COPY ./web .
COPY ./VERSION .



RUN bun add -d rollup@4.32.1

# ✅ 可选：验证版本（可删除）
RUN bun x rollup --version


RUN DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat VERSION) bun run build

FROM golang:alpine AS builder2

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux

WORKDIR /build

ADD go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=builder /build/dist ./web/dist
RUN go build -ldflags "-s -w -X 'one-api/common.Version=$(cat VERSION)'" -o one-api

FROM alpine

RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates tzdata ffmpeg \
    && update-ca-certificates

COPY --from=builder2 /build/one-api /
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/one-api"]
