# syntax=docker/dockerfile:1.7

FROM golang:1.26.1-alpine AS builder

ARG GOOSE_VERSION=v3.27.0

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/rota-server ./cmd/server
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOBIN=/out go install -tags="no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb" github.com/pressly/goose/v3/cmd/goose@${GOOSE_VERSION}

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=busybox:1.37.0-uclibc /bin/busybox /busybox
COPY --from=builder /out/rota-server /usr/local/bin/rota-server
COPY --from=builder /out/goose /usr/local/bin/goose

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/rota-server"]
