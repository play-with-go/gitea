# golang:1.19.1
FROM golang@sha256:2d17ffd12a2cdb25d4a633ad25f8dc29608ed84f31b3b983427d825280427095 AS stage1

ENV GOCACHE=/root/.cache/go/gocache
ENV GOMODCACHE=/root/.cache/go/gomodcache
ENV CGO_ENABLED=0
ENV GOPATH=

COPY . .

RUN --mount=type=cache,target=/root/.cache/go go build -o cmd/gitea/gitea ./cmd/gitea

# buildpack-deps:bullseye-scm - because for now we need ssh-keyscan
FROM buildpack-deps@sha256:ae1f98a016484d09849a53b809f1a55e177f6c3cb7f6c78d4bd760d44e7dab69

RUN mkdir /runbin

COPY --from=stage1 /go/cmd/gitea/gitea /runbin
COPY ./docker/cmd_gitea/playwithgo-entrypoint.sh /playwithgo-entrypoint.sh

ENTRYPOINT ["/playwithgo-entrypoint.sh"]
