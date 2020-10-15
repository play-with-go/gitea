# syntax = docker/dockerfile:experimental@sha256:de85b2f3a3e8a2f7fe48e8e84a65f6fdd5cd5183afa6412fff9caa6871649c44

# golang:1.15.2
FROM golang@sha256:da7ff43658854148b401f24075c0aa390e3b52187ab67cab0043f2b15e754a68 AS stage1

ENV GOCACHE=/root/.cache/go/gocache
ENV GOMODCACHE=/root/.cache/go/gomodcache
ENV CGO_ENABLED=0
ENV GOPATH=

COPY . .

RUN --mount=type=cache,target=/root/.cache/go go build -o cmd/gitea/gitea ./cmd/gitea

# busybox:1.32.0
FROM busybox@sha256:d9ce5c47b31f3ca5626a3afe8e17c69169eef194ba8f9d68ade898759f49c907

RUN mkdir /runbin

COPY --from=stage1 /go/cmd/gitea/gitea /runbin
COPY ./docker/cmd_gitea/playwithgo-entrypoint.sh /playwithgo-entrypoint.sh

ENTRYPOINT ["/playwithgo-entrypoint.sh"]
