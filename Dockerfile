FROM gcr.io/distroless/static-debian13:nonroot@sha256:8ce6dd87e9823c5b48dcfb2c8604714ccb351ce59607327bcc4957448da279b5
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/twitch_exporter /bin/twitch_exporter
EXPOSE 9184
USER nonroot
ENTRYPOINT ["/bin/twitch_exporter"]
