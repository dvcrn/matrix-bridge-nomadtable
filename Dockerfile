FROM golang:1.25-alpine3.21 AS builder

RUN apk add --no-cache git ca-certificates build-base su-exec olm-dev

COPY . /build
WORKDIR /build

ENV GOCACHE=/root/.cache/go-build

RUN --mount=type=cache,target="/root/.cache/go-build" go build -o /usr/bin/matrix-nomadtable .


FROM alpine:3.21

WORKDIR /app

ENV UID=1337 \
    GID=1337

RUN apk add --no-cache su-exec ca-certificates olm bash jq yq curl

COPY --from=builder /usr/bin/matrix-nomadtable /usr/bin/matrix-nomadtable

VOLUME /data

CMD ["/usr/bin/matrix-nomadtable"]
