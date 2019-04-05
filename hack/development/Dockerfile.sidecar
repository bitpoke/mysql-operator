# Copy the mysql-operator-sidecar into it's own image
# NOTE: this image is for development only
FROM debian:stretch-slim as sidecar

RUN groupadd -g 999 mysql
RUN useradd -u 999 -r -g 999 -s /sbin/nologin \
    -c "Default Application User" mysql

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        apt-transport-https ca-certificates wget \
        gnupg1 dirmngr \
    && rm -rf /var/lib/apt/lists/*

RUN apt-key adv --keyserver ha.pool.sks-keyservers.net --recv-keys 9334A25F8507EFA5

RUN echo 'deb https://repo.percona.com/apt stretch main' > /etc/apt/sources.list.d/percona.list

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        percona-toolkit percona-xtrabackup-24 unzip \
    && wget https://github.com/maxbube/mydumper/releases/download/v0.9.5/mydumper_0.9.5-2.stretch_amd64.deb \
    && dpkg -i mydumper_0.9.5-2.stretch_amd64.deb \
    && rm -rf mydumper_0.9.5-2.stretch_amd64.deb /var/lib/apt/lists/* \
    && wget https://downloads.rclone.org/rclone-current-linux-amd64.zip \
    && unzip rclone-current-linux-amd64.zip \
    && mv rclone-*-linux-amd64/rclone /usr/local/bin/ \
    && rm -rf rclone-*-linux-amd64 rclone-current-linux-amd64.zip \
    && chmod 755 /usr/local/bin/rclone

USER mysql

# set expiration time for dev images
# https://support.coreos.com/hc/en-us/articles/115001384693-Tag-Expiration
LABEL quay.expires-after=2d

COPY ./hack/docker/sidecar-entrypoint.sh /usr/local/bin/sidecar-entrypoint.sh
COPY ./bin/mysql-operator-sidecar_linux_amd64 /usr/local/bin/mysql-operator-sidecar
ENTRYPOINT ["/usr/local/bin/sidecar-entrypoint.sh"]
