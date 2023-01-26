###############################################################################
#  Docker image for sidecar containers
###############################################################################

FROM debian:buster-slim as sidecar

ARG USER_ID=999
RUN groupadd -g ${USER_ID} mysql
RUN useradd -u ${USER_ID} -r -g ${USER_ID} -s /sbin/nologin \
    -c "Default Application User" mysql

COPY rootfs/ /

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        apt-transport-https ca-certificates unzip pigz wget gnupg \
    && rm -rf /var/lib/apt/lists/*

RUN export RCLONE_VERSION=1.57.0 \
    && gpg --import /etc/gpg-keys/rclone.gpg \
    && wget -nv https://github.com/ncw/rclone/releases/download/v${RCLONE_VERSION}/rclone-v${RCLONE_VERSION}-linux-amd64.zip \
    && wget -nv https://github.com/ncw/rclone/releases/download/v${RCLONE_VERSION}/SHA256SUMS \
    && gpg --verify --output=- SHA256SUMS > sums \
    && sha256sum -c --ignore-missing sums \
    && unzip rclone-*-linux-amd64.zip \
    && mv rclone-*-linux-amd64/rclone /usr/local/bin/ \
    && chmod 755 /usr/local/bin/rclone \
    && rm -r rclone-*-linux-amd64 rclone-*-linux-amd64.zip

RUN echo 'deb https://repo.percona.com/apt buster main' > /etc/apt/sources.list.d/percona.list

ARG XTRABACKUP_PKG=percona-xtrabackup-24
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        percona-toolkit ${XTRABACKUP_PKG} unzip default-mysql-client \
    && rm -rf /var/lib/apt/lists/*

USER mysql
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
