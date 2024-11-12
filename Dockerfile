# build stage
ARG HTTP_PROXY
ARG HTTPS_PROXY
FROM golang:1.23 AS build-env
WORKDIR /app
COPY  . /app
RUN useradd -u 10001 webhook && \
    go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o cosignwebhook

FROM alpine:latest
COPY --from=build-env /app/cosignwebhook /cosignwebhook
COPY --from=build-env /etc/passwd /etc/passwd
USER webhook
ENTRYPOINT ["/cosignwebhook"]
