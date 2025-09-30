FROM scratch
COPY wcfc-updater /wcfc-updater
COPY config.yml /config.yml
ENTRYPOINT ["/wcfc-updater"]
