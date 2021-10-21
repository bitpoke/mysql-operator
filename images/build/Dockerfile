# Copyright 2016 The Upbound Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM google/cloud-sdk:357.0.0

ARG ARCH

ENV DEBIAN_FRONTEND noninteractive

COPY build-scripts /usr/local/build-scripts

COPY --from=docker/buildx-bin:0.6.1 /buildx /usr/libexec/docker/cli-plugins/docker-buildx
RUN docker buildx version

# ------------------------------------------------------------------------------------------------
# install build and release tools
RUN /usr/local/build-scripts/install-packages.sh \
        apt-transport-https \
        gettext \
        jq \
        lsb-release \
        make \
        rsync \
        runit \
        sudo \
        wget \
        zip
# ------------------------------------------------------------------------------------------------
# PHP
RUN sh -c 'echo "deb https://packages.sury.org/php/ $(lsb_release -sc) main" > /etc/apt/sources.list.d/php.list' \
    && wget -O /etc/apt/trusted.gpg.d/php.gpg https://packages.sury.org/php/apt.gpg \
    && /usr/local/build-scripts/install-packages.sh \
        php7.4-bcmath \
        php7.4-curl \
        php7.4-cli \
        php7.4-fpm \
        php7.4-gd \
        php7.4-mbstring \
        php7.4-mysql \
        php7.4-opcache \
        php7.4-tidy \
        php7.4-xml \
        php7.4-xmlrpc \
        php7.4-xsl \
        php7.4-zip \
        php-apcu \
        php-apcu-bc \
        php-geoip \
        php-imagick \
        php-memcached \
        php-redis \
        php-sodium \
        php-yaml \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# ------------------------------------------------------------------------------------------------
# git config
RUN set -ex \
    && git config --global user.email "bot@bitpoke.cloud" \
    && git config --global user.name "Bitpoke Bot" \
    && git config --global diff.tar-filelist.binary true \
    && git config --global diff.tar-filelist.textconv 'tar -tvf' \
    && git config --global diff.tar.binary true \
    && git config --global diff.tar.textconv 'tar -xvOf'

# ------------------------------------------------------------------------------------------------
# Go support
RUN GO_VERSION=1.16.7 && \
    GO_HASH=7fe7a73f55ba3e2285da36f8b085e5c0159e9564ef5f63ee0ed6b818ade8ef04 && \
    curl -fsSL https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz -o golang.tar.gz && \
    echo "${GO_HASH}  golang.tar.gz" | sha256sum -c - && \
    tar -C /usr/local -xzf golang.tar.gz && \
    rm golang.tar.gz
ENV GOPATH /workspace
ENV PATH /workspace/bin:/usr/local/go/bin:$PATH

# precompile the go standard library for all supported platforms and configurations
# the install suffixes match those in golang.mk so please keep them in sync
RUN platforms="darwin_amd64 windows_amd64 linux_amd64 linux_arm64" && \
    for p in $platforms; do CGO_ENABLED=0 GOOS=${p%_*} GOARCH=${p##*_} GOARM=7 go install -installsuffix static -a std; done

# ------------------------------------------------------------------------------------------------
# Node JS and chrome support
RUN curl -fsSL https://deb.nodesource.com/setup_14.x | bash - && \
    curl -fsSL https://dl-ssl.google.com/linux/linux_signing_key.pub | apt-key add - && \
    echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" > /etc/apt/sources.list.d/google.list && \
    /usr/local/build-scripts/install-packages.sh \
        nodejs \
        google-chrome-stable \
        xvfb && \
    rm -f /etc/apt/sources.list.d/google.list && \
    ln -fs /usr/local/bin/xvfb-chrome /usr/bin/google-chrome && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
ENV CHROME_BIN /usr/bin/google-chrome

# ------------------------------------------------------------------------------------------------
# Yarn
RUN curl -sS https://dl.yarnpkg.com/debian/pubkey.gpg | sudo apt-key add - && \
    echo "deb https://dl.yarnpkg.com/debian/ stable main" | sudo tee /etc/apt/sources.list.d/yarn.list && \
    /usr/local/build-scripts/install-packages.sh \
        yarn && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# ------------------------------------------------------------------------------------------------
# rclone
RUN set -ex \
    && export RCLONE_VERSION=1.56.0 \
    && curl -sL -o rclone-v${RCLONE_VERSION}-linux-amd64.deb https://github.com/rclone/rclone/releases/download/v${RCLONE_VERSION}/rclone-v${RCLONE_VERSION}-linux-amd64.deb \
    && dpkg -i rclone-v${RCLONE_VERSION}-linux-amd64.deb \
    && rm rclone-v${RCLONE_VERSION}-linux-amd64.deb \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# ------------------------------------------------------------------------------------------------
# dockerize
RUN set -ex \
    && export DOCKERIZE_VERSION="2.1.0" \
    && curl -sL -o dockerize.tar.gz "https://github.com/presslabs/dockerize/releases/download/v${DOCKERIZE_VERSION}/dockerize-linux-amd64-v${DOCKERIZE_VERSION}.tar.gz" \
    && tar -C /usr/local/bin -xzvf dockerize.tar.gz \
    && rm dockerize.tar.gz \
    && chmod 0755 /usr/local/bin/dockerize \
    && chown root:root /usr/local/bin/dockerize

# ------------------------------------------------------------------------------------------------
# sops
RUN set -ex \
    && export SOPS_VERSION="3.7.1" \
    && curl -sL -o /usr/local/bin/sops "https://github.com/mozilla/sops/releases/download/v${SOPS_VERSION}/sops-v${SOPS_VERSION}.linux" \
    && chmod 0755 /usr/local/bin/sops \
    && chown root:root /usr/local/bin/sops

# ------------------------------------------------------------------------------------------------
# helm
RUN set -ex \
    && export HELM_VERSION="3.6.3" \
    && curl -sL -o helm.tar.gz "https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz" \
    && tar -C /usr/local/bin -xzvf helm.tar.gz --strip-components 1 linux-amd64/helm \
    && rm helm.tar.gz \
    && chmod 0755 /usr/local/bin/helm \
    && chown root:root /usr/local/bin/helm

# ------------------------------------------------------------------------------------------------
# helm secrets plugin
RUN set -ex \
    && export HELM_SECRETS_VERSION="3.8.3" \
    && helm plugin install https://github.com/jkroepke/helm-secrets --version ${HELM_SECRETS_VERSION}

# ------------------------------------------------------------------------------------------------
# kustomize
RUN set -ex \
    && export KUSTOMIZE_VERSION="4.2.0" \
    && curl -sL -o kustomize.tar.gz "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_amd64.tar.gz" \
    && tar -C /usr/local/bin -xzvf kustomize.tar.gz \
    && rm kustomize.tar.gz \
    && chmod 0755 /usr/local/bin/kustomize \
    && chown root:root /usr/local/bin/kustomize

# ------------------------------------------------------------------------------------------------
# docker-compose

RUN set -ex \
    && export DOCKER_COMPOSE_VERSION="1.29.2" \
    && curl -sL -o /usr/local/bin/docker-compose "https://github.com/docker/compose/releases/download/${DOCKER_COMPOSE_VERSION}/docker-compose-Linux-x86_64" \
    && chmod +x /usr/local/bin/docker-compose \
    && chown root:root /usr/local/bin/docker

# ------------------------------------------------------------------------------------------------
# Run tini as PID 1 and avoid signal handling issues

RUN set -ex \
    && export TINI_VERSION=v0.19.0 \
    && curl -sL -o /tini https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini-${ARCH} \
    && chmod +x /tini

COPY root /

ENTRYPOINT [ "/tini", "-g", "--", "/build/run.sh" ]
