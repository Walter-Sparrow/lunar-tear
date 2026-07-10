FROM alpine:latest AS builder

RUN apk add --no-cache \
    protobuf \
    protobuf-dev \
    protoc \
    make \
    go \
    libcap

RUN --mount=type=cache,target=/root/go/pkg/mod \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
    go install github.com/pressly/goose/v3/cmd/goose@latest

WORKDIR /usr/local/src
COPY . .

RUN --mount=type=cache,target=/root/go/pkg/mod \
    PATH="$PATH:$(go env GOPATH)/bin" make proto && \
    go build -o lunar-tear ./cmd/lunar-tear && \
    setcap cap_net_bind_service=+ep ./lunar-tear

FROM alpine:latest

WORKDIR /opt/lunar-tear

RUN chown 1000:1000 /opt/lunar-tear

USER 1000

COPY --from=builder /usr/local/src/lunar-tear .
COPY --from=builder /root/go/bin/goose /usr/local/bin/goose
COPY --from=builder /usr/local/src/migrations ./migrations

COPY entrypoint.sh .

ENTRYPOINT /opt/lunar-tear/entrypoint.sh
