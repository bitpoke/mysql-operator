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

RUN set -ex \
    && export ORCHESTRATOR_VERSION=3.2.6 \
    && wget https://github.com/openark/orchestrator/releases/download/v${ORCHESTRATOR_VERSION}/orchestrator-${ORCHESTRATOR_VERSION}-linux-amd64.tar.gz -O- | \
        tar -C / -xzv

COPY rootfs/ /
RUN set -ex \
    && mkdir -p /etc/orchestrator /var/lib/orchestrator \
    && chown -R 65532:65532 /etc/orchestrator /var/lib/orchestrator

# switch back to nonroot for runtime
USER 65532
EXPOSE 3000 10008
VOLUME [ "/var/lib/orchestrator" ]

WORKDIR "/usr/local/orchestrator"

ENTRYPOINT [ "/usr/local/bin/dockerize", \
             "-no-overwrite", \
             "-template", \
             "/usr/local/share/orchestrator/templates/orchestrator.conf.json:/etc/orchestrator/orchestrator.conf.json", \
             "-template", \
             "/usr/local/share/orchestrator/templates/orc-topology.cnf:/etc/orchestrator/orc-topology.cnf", \
             "--" ]
CMD ["/usr/local/orchestrator/orchestrator", "-config", "/etc/orchestrator/orchestrator.conf.json", "http"]
