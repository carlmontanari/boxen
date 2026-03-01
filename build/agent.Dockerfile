ARG VERSION=0.0.0

FROM golang:1.26-trixie AS builder

WORKDIR /boxen
COPY . .

RUN go mod download

RUN CGO_ENABLED=0 \
    go run \
    build/write-libscrapli-to-cache/main.go

RUN cp /root/.cache/scrapli/libscrapli.so.* /root/.cache/scrapli/libscrapli.so

RUN CGO_ENABLED=0 \
    go build \
    -ldflags "-s -w -X github.com/carlmontanari/boxen/constants.Version=${VERSION}" \
    -trimpath \
    -a \
    -o \
    out/boxen \
    cmd/main.go


FROM debian:bookworm-slim

ENV LIBSCRAPLI_PATH=/boxen/.libscrapli.so

RUN apt-get update && \
    apt-get install -yq --no-install-recommends \
    ca-certificates \
    bridge-utils \
    iproute2 \
    socat \
    qemu-utils \
    qemu-system-x86 \
    libguestfs-tools \
    curl \
    vim \
    telnet \
    tcpdump \
    procps \
    openssh-client \
    inetutils-ping \
    traceroute \
    genisoimage && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* /var/cache/apt/archive/*.deb

WORKDIR /boxen

COPY build/tc-tap-ifup /etc/
RUN chmod 0777 /etc/tc-tap-ifup

COPY build/scrapligo_definition.yaml .scrapligo_definition.yaml

COPY --from=builder /root/.cache/scrapli/libscrapli.so /boxen/.libscrapli.so
COPY --from=builder /boxen/out/boxen /boxen/boxen

ENTRYPOINT ["/boxen/boxen", "package"]
