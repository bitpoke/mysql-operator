FROM quay.io/presslabs/mysql-toolbox:v0.1

COPY docker-entrypoint.sh mysql-operator-sidecar /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
