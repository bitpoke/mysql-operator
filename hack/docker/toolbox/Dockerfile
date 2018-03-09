FROM debian:stretch

RUN apt-get update && apt-get install -y \
    wget curl alien unzip mysql-client

# https://www.percona.com/doc/percona-xtrabackup/LATEST/installation/apt_repo.html
RUN wget https://repo.percona.com/apt/percona-release_0.1-5.stretch_all.deb -P /tmp/ \
    && dpkg -i /tmp/percona-release_0.1-5.stretch_all.deb \
    && rm /tmp/percona-release_0.1-5.stretch_all.deb

RUN apt-get update && apt-get install -y \
  percona-xtrabackup-24 \
  && rm -rf /var/lib/apt/lists/*

RUN wget https://nmap.org/dist/ncat-7.60-1.x86_64.rpm \
    && alien ncat-7.60-1.x86_64.rpm \
    && dpkg --install ncat_7.60-2_amd64.deb \
    && wget https://downloads.rclone.org/rclone-current-linux-amd64.zip \
    && unzip rclone-current-linux-amd64.zip \
    && mv rclone-*-linux-amd64/rclone /usr/bin/ \
    && chmod 755 /usr/bin/rclone

WORKDIR /app

COPY ./hack/docker/toolbox/docker-entrypoint.sh /app/
COPY ./bin/linux/toolbox /usr/bin/

ENTRYPOINT ["/app/docker-entrypoint.sh"]