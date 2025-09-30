FROM scratch
COPY wcfc-updater /wcfc-updater
COPY config.yml /config.yml
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENTRYPOINT ["/wcfc-updater"]
