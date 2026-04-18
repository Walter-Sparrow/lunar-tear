# First, generate the protos only if they change
FROM bufbuild/buf AS proto-builder

WORKDIR /usr/local/src
COPY buf.yaml buf.gen.yaml ./
COPY proto/ ./proto/

RUN buf generate


# Then build the server
FROM golang:1.24-alpine AS builder

WORKDIR /usr/local/src
COPY --from=proto-builder /usr/local/src/gen ./gen
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o lunar-tear ./cmd/lunar-tear

# Wish it was a better way to do this.
# I'd suggest not binding to port 443 and using a proxy like nginx to handle that.
# Yeah, I know, too complicated, but fuck I hate doing this.
FROM builder AS security-flaw

RUN apk add --no-cache libcap
RUN setcap cap_net_bind_service=+ep ./lunar-tear

# Hardened final image
FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /opt/lunar-tear
USER 65532:65532
COPY --from=security-flaw /usr/local/src/lunar-tear .

# IMHO: if the entrypoint is a single line, you're introducing unnecesary bloat
# Chose between env variables or input arguments, don't use the former to feed 
# the latter.
ENTRYPOINT [ "./lunar-tear" ] 
