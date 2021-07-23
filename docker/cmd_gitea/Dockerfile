# syntax = docker/dockerfile:experimental@sha256:de85b2f3a3e8a2f7fe48e8e84a65f6fdd5cd5183afa6412fff9caa6871649c44

# golang:1.16.6
FROM golang@sha256:4544ae57fc735d7e415603d194d9fb09589b8ad7acd4d66e928eabfb1ed85ff1 AS stage1

ENV GOCACHE=/root/.cache/go/gocache
ENV GOMODCACHE=/root/.cache/go/gomodcache
ENV CGO_ENABLED=0
ENV GOPATH=

COPY . .

RUN --mount=type=cache,target=/root/.cache/go go build -o cmd/gitea/gitea ./cmd/gitea

# buildpack-deps:buster-scm - because we need ssh-keyscan
FROM buildpack-deps@sha256:f010c9887ecea10d052247ef8befb79c2d17ea3f92ab6cb05039caab757fb14e

RUN mkdir /runbin

COPY --from=stage1 /go/cmd/gitea/gitea /runbin
COPY ./docker/cmd_gitea/playwithgo-entrypoint.sh /playwithgo-entrypoint.sh

ENTRYPOINT ["/playwithgo-entrypoint.sh"]
