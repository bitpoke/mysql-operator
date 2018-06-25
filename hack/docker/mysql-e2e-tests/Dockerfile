FROM golang:1.9

# install kubectl
ENV KUBECTL_VERSION 1.9.8
RUN wget -q https://storage.googleapis.com/kubernetes-release/release/v$KUBECTL_VERSION/bin/linux/amd64/kubectl -O/usr/local/bin/kubectl \
    && chmod 0755 /usr/local/bin/kubectl \
    && chown root:root /usr/local/bin/kubectl

# install kubernetes helm
ENV HELM_VERSION 2.9.1
RUN wget -q https://kubernetes-helm.storage.googleapis.com/helm-v$HELM_VERSION-linux-amd64.tar.gz \
    && tar -C /usr/local/bin -xzvf helm-v$HELM_VERSION-linux-amd64.tar.gz --strip-components 1 linux-amd64/helm \
    && rm helm-v$HELM_VERSION-linux-amd64.tar.gz \
    && chmod 0755 /usr/local/bin/helm \
    && chown root:root /usr/local/bin/helm