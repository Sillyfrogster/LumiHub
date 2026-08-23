# syntax=docker/dockerfile:1.7

FROM golang:1.26.5-alpine3.23 AS build

WORKDIR /src/api

COPY api/go.mod api/go.sum ./
RUN go mod download
RUN CGO_ENABLED=0 GOBIN=/out go install \
    -tags="no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb" \
    github.com/pressly/goose/v3/cmd/goose@v3.26.0

COPY api/ ./

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/backup ./cmd/backup

FROM alpine:3.23

RUN apk add --no-cache ca-certificates postgresql-client restic \
    && addgroup -g 10001 illarin \
    && adduser -D -H -u 10001 -G illarin illarin

WORKDIR /app

COPY --from=build /out/server /app/server
COPY --from=build /out/backup /app/backup
COPY --from=build /out/goose /app/goose
COPY api/migrations /app/migrations

USER 10001:10001

EXPOSE 8080

ENTRYPOINT ["/app/server"]
