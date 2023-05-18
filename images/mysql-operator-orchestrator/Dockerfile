FROM golang:1.19.9-bullseye as builder

RUN apt-get update
RUN apt-get install -y ruby ruby-dev rubygems build-essential
RUN gem install --no-document fpm
ENV GOPATH=/tmp/go

RUN apt-get install -y curl rsync gcc g++ bash git tar rpm

RUN mkdir -p $GOPATH/src/github.com/openark/orchestrator
WORKDIR $GOPATH/src/github.com/openark/orchestrator
RUN git clone https://github.com/percona/orchestrator.git . && git checkout v3.2.6-8
RUN bash build.sh -b -P
RUN set -ex \
    && mv build /tmp \
    && mkdir -p /tmp/build/share/orchestrator/resources \
    # copy web static files and templates
    && cp -a resources/templates resources/public /tmp/build/share/orchestrator/resources \
    # copy orchestrator cli
    && cp resources/bin/orchestrator-client /tmp/build/bin/orchestrator-cli && chmod +x /tmp/build/bin/orchestrator-cli

# Docker image for orchestrator
# The base image is pinned to the debug-nonroot tag
FROM gcr.io/distroless/base-debian11@sha256:e76722f06f7c15e0076072fb02782ec59923b0d658b8a3d80bb79deaee6fb44d
SHELL ["/busybox/sh", "-c"]

# switch to root for installing software
USER root

RUN set -ex \
    && mkdir -p /usr/local/bin \
    && export DOCKERIZE_VERSION=0.6.1 \
    && wget https://github.com/jwilder/dockerize/releases/download/v${DOCKERIZE_VERSION}/dockerize-linux-amd64-v${DOCKERIZE_VERSION}.tar.gz -O- | \
        tar -C /usr/local/bin -xzv

COPY rootfs/ /
COPY --from=builder --chown=root:root /tmp/build/ /usr/local/
RUN set -ex \
    && mkdir -p /etc/orchestrator /var/lib/orchestrator \
    && chown -R 65532:65532 /etc/orchestrator /var/lib/orchestrator

WORKDIR "/usr/local/share/orchestrator"

# switch back to nonroot for runtime
USER 65532
EXPOSE 3000 10008
VOLUME [ "/var/lib/orchestrator" ]


ENTRYPOINT [ "/usr/local/bin/dockerize", \
             "-no-overwrite", \
             "-template", \
             "/usr/local/share/orchestrator/templates/orchestrator.conf.json:/etc/orchestrator/orchestrator.conf.json", \
             "-template", \
             "/usr/local/share/orchestrator/templates/orc-topology.cnf:/etc/orchestrator/orc-topology.cnf", \
             "--" ]
CMD ["/usr/local/bin/orchestrator", "-quiet", "-config", "/etc/orchestrator/orchestrator.conf.json", "http"]
