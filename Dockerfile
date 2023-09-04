# build stage
FROM golang:1.21 AS build-env
WORKDIR /app
COPY  . /app
RUN useradd -u 10001 webhook
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o cosignwebhook

FROM alpine:latest
COPY --from=build-env /app .
COPY --from=build-env /etc/passwd /etc/passwd
USER webhook
ENTRYPOINT ["/cosignwebhook"]
