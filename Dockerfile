FROM alpine:3.6

ADD ./bin/linux/operator /usr/bin/titanium-operator

ENTRYPOINT ["/usr/bin/titanium-operator"]