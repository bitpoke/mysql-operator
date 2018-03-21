FROM alpine:3.6

ADD ./bin/linux/titanium /usr/bin/titanium-operator


ENTRYPOINT ["/usr/bin/titanium-operator"]
