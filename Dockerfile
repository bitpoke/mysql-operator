FROM alpine:3.6

ADD ./bin/linux/operator /usr/bin/mysql-operator

ENTRYPOINT ["/usr/bin/mysql-operator"]
