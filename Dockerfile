FROM gcr.io/distroless/static-debian12:nonroot
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/twitch_exporter /bin/twitch_exporter
EXPOSE 9184
USER nonroot
ENTRYPOINT ["/bin/twitch_exporter"]
