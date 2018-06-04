FROM docker:18.03

RUN set -ex; \
    apk add --no-cache \
        bash \
        curl \
    ;
    

#ENV KUBECTL_VERSION v1.10.3
#ADD https://storage.googleapis.com/kubernetes-release/release/$KUBECTL_VERSION/bin/linux/amd64/kubectl /usr/local/bin/kubectl

#RUN chmod +x /usr/local/bin/kubectl

CMD ["bash"]