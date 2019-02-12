FROM scratch

# set expiration time for dev images
# https://support.coreos.com/hc/en-us/articles/115001384693-Tag-Expiration
LABEL quay.expires-after=2d

COPY ./bin/mysql-operator_linux_amd64 /mysql-operator
ENTRYPOINT ["/mysql-operator"]
