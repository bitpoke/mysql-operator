###############################################################################
#  Docker image for sidecar containers
###############################################################################

FROM debian:bullseye-slim as sidecar

RUN groupadd -g 999 mysql
RUN useradd -u 999 -r -g 999 -s /sbin/nologin \
    -c "Default Application User" mysql

COPY rootfs/ /

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates rsync unzip pigz wget curl gnupg \
        default-mysql-client \
        libaio1 \
        libcurl4-openssl-dev \
        libdbd-mysql-perl \
        libev4 \
        libio-socket-ssl-perl \
        libprocps8 \
        libterm-readkey-perl \
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

ARG PERCONA_TOOLKIT_URL=https://downloads.percona.com/downloads/percona-toolkit/3.5.2/binary/debian/bullseye/x86_64/percona-toolkit_3.5.2-2.bullseye_amd64.deb
ARG PERCONA_XTRABACKUP_URL=https://downloads.percona.com/downloads/Percona-XtraBackup-2.4/Percona-XtraBackup-2.4.26/binary/debian/bullseye/x86_64/percona-xtrabackup-24_2.4.26-1.bullseye_amd64.deb
RUN set -ex \
    && mkdir -p /tmp/dl && cd /tmp/dl \
    && wget ${PERCONA_TOOLKIT_URL} \
    && wget ${PERCONA_XTRABACKUP_URL} \
    && dpkg -i *.deb \
    && cd / && rm -rf /tmp/dl

USER mysql
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
