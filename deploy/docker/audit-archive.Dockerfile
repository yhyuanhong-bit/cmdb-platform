# Multi-stage build for the audit-archive CLI. The runtime image
# needs `psql` because the CronJob's inner script issues the
# CREATE TABLE IF NOT EXISTS step before invoking the Go binary.
# We could push that SQL into the binary, but keeping it shell-side
# lets ops override the partition template without a redeploy.

FROM golang:1.25 AS build
WORKDIR /src
COPY cmdb-core/go.mod cmdb-core/go.sum ./
RUN go mod download
COPY cmdb-core/. .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w" \
    -o /out/audit-archive ./cmd/audit-archive

FROM alpine:3.20
RUN apk add --no-cache postgresql-client ca-certificates tzdata \
 && addgroup -g 65532 nonroot \
 && adduser -u 65532 -G nonroot -s /sbin/nologin -D nonroot
COPY --from=build /out/audit-archive /usr/local/bin/audit-archive
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/audit-archive"]
