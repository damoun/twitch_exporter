ARG ARCH="amd64"
ARG OS="linux"
FROM quay.io/prometheus/busybox-${OS}-${ARCH}:latest

ARG ARCH="amd64"
ARG OS="linux"
COPY .build/${OS}-${ARCH}/twitch_exporter   /bin/twitch_exporter

EXPOSE     9184
ENTRYPOINT [ "/bin/twitch_exporter" ]