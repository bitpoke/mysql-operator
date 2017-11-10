FROM python:3.6-stretch

# https://www.percona.com/doc/percona-xtrabackup/LATEST/installation/apt_repo.html
RUN wget https://repo.percona.com/apt/percona-release_0.1-5.stretch_all.deb \
    && dpkg -i percona-release_0.1-5.stretch_all.deb

RUN apt-get update && apt-get install -y \
  --no-install-suggests curl mysql-client percona-xtrabackup-24 \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY ./ /app/

RUN pip install -r requirements.txt
RUN pip install -e .