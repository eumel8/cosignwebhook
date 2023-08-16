# build stage
FROM golang:1.21 AS build-env
RUN mkdir -p /go/src/github.com/eumel8/cosignwebhook
WORKDIR /go/src/github.com/eumel8/cosignwebhook
COPY  . .
RUN useradd -u 10001 webhook
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o cosignwebhook

#FROM scratch
FROM alpine:latest
COPY --from=build-env /go/src/github.com/eumel8/cosignwebhook/cosignwebhook .
COPY --from=build-env /etc/passwd /etc/passwd
USER webhook
ENTRYPOINT ["/cosignwebhook"]
