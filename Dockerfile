FROM gcr.io/distroless/static-debian12:nonroot
COPY twitch_exporter /bin/twitch_exporter
EXPOSE 9184
USER nonroot
ENTRYPOINT ["/bin/twitch_exporter"]
