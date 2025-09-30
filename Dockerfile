FROM registry.fedoraproject.org/fedora-minimal:latest
COPY wcfc-updater /wcfc-updater
COPY config.yml /config.yml
ENTRYPOINT ["/wcfc-updater"]
