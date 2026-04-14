FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /bin/cmdb-core ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/cmdb-core /usr/local/bin/cmdb-core
COPY --from=builder /src/migrations /migrations

ENV MIGRATIONS_DIR=/migrations

EXPOSE 8080

ENTRYPOINT ["cmdb-core"]
