FROM python:3.6-stretch

# https://www.percona.com/doc/percona-xtrabackup/LATEST/installation/apt_repo.html
RUN wget https://repo.percona.com/apt/percona-release_0.1-5.stretch_all.deb \
    && dpkg -i percona-release_0.1-5.stretch_all.deb

RUN apt-get update && apt-get install -y \
  curl alien unzip mysql-client percona-xtrabackup-24 \
  && rm -rf /var/lib/apt/lists/*

RUN wget https://nmap.org/dist/ncat-7.60-1.x86_64.rpm \
    && alien ncat-7.60-1.x86_64.rpm \
    && dpkg --install ncat_7.60-2_amd64.deb \
    && wget https://downloads.rclone.org/rclone-current-linux-amd64.zip \
    && unzip rclone-current-linux-amd64.zip \
    && mv rclone-*-linux-amd64/rclone /usr/bin/ \
    && chmod 755 /usr/bin/rclone

WORKDIR /app

COPY ./ /app/

RUN pip install -r requirements.txt
RUN pip install -e .