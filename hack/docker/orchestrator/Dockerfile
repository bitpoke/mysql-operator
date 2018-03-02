FROM alpine

EXPOSE 3000

ENV GOPATH=/tmp/go
ENV DOCKERIZE_VERSION v0.6.0

RUN set -ex \
    && apk add --update --no-cache bash \
    && apk add --update --no-cache --virtual .build-deps \
        rsync \
        git \
        go \
        build-base \
        wget \
        tar \
        openssl \
    && cd /tmp \
    && wget https://github.com/jwilder/dockerize/releases/download/$DOCKERIZE_VERSION/dockerize-alpine-linux-amd64-$DOCKERIZE_VERSION.tar.gz \
    && tar -C /usr/local/bin -xzvf dockerize-alpine-linux-amd64-$DOCKERIZE_VERSION.tar.gz \
    && { go get -d github.com/github/orchestrator ; : ; } \
    && cd $GOPATH/src/github.com/github/orchestrator \
    && bash build.sh -b -p / \
    && rsync -av $(find /tmp/orchestrator-release -type d -name orchestrator -maxdepth 2)/ / \
    && rsync -av $(find /tmp/orchestrator-release -type d -name orchestrator-cli -maxdepth 2)/ / \
    && cd / \
    && mkdir /orchestrator/conf \
    && apk del .build-deps \
    && rm -rf /tmp/*

WORKDIR /orchestrator

COPY orchestrator.conf.json.tpl /conf-tpl/
COPY orc-topology.cnf.tpl /conf-tpl/

VOLUME [ "/var/lib/orchestrator" ]

ENTRYPOINT ["dockerize"]
CMD ["-template", "/conf-tpl/orchestrator.conf.json.tpl:/orchestrator/conf/orchestrator.conf.json", \
     "-template", "/conf-tpl/orc-topology.cnf.tpl:/orchestrator/conf/orc-topology.cnf", \
     "-delims", "<%:%>", \
     "/orchestrator/orchestrator", "http"]