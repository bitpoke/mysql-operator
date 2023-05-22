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

FROM google/cloud-sdk:431.0.0

ARG ARCH

ENV DEBIAN_FRONTEND noninteractive

COPY build-scripts /usr/local/build-scripts

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
# Go support
RUN GO_VERSION=1.17.13 && \
    GO_HASH=4cdd2bc664724dc7db94ad51b503512c5ae7220951cac568120f64f8e94399fc && \
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
        php7.4-apcu \
        php7.4-apcu-bc \
        php7.4-geoip \
        php7.4-imagick \
        php7.4-memcached \
        php7.4-redis \
        php7.4-yaml \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* \
    && COMPOSER_VERSION=2.2.21 \
    && COMPOSER_HASH=5211584ad39af26704da9f6209bc5d8104a2d576e80ce9c7ed8368ddd779d0af \
    && curl -fsSL -o /usr/local/bin/composer https://getcomposer.org/download/${COMPOSER_VERSION}/composer.phar \
    && echo "${COMPOSER_HASH}  /usr/local/bin/composer" | sha256sum -c - \
    && chmod +x /usr/local/bin/composer

# ------------------------------------------------------------------------------------------------
# docker-compose and docker-buildx

COPY --from=docker/buildx-bin:0.10.4 /buildx /usr/libexec/docker/cli-plugins/docker-buildx
RUN set -ex \
    && export DOCKER_COMPOSE_VERSION="2.18.1" \
    && curl -sL -o /usr/libexec/docker/cli-plugins/docker-compose "https://github.com/docker/compose/releases/download/v${DOCKER_COMPOSE_VERSION}/docker-compose-linux-x86_64" \
    && chmod 0755 /usr/libexec/docker/cli-plugins/docker-compose

# ------------------------------------------------------------------------------------------------
# rclone
RUN set -ex \
    && export RCLONE_VERSION=1.62.2 \
    && curl -sL -o rclone-v${RCLONE_VERSION}-linux-amd64.deb https://github.com/rclone/rclone/releases/download/v${RCLONE_VERSION}/rclone-v${RCLONE_VERSION}-linux-amd64.deb \
    && dpkg -i rclone-v${RCLONE_VERSION}-linux-amd64.deb \
    && rm rclone-v${RCLONE_VERSION}-linux-amd64.deb \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# ------------------------------------------------------------------------------------------------
# dockerize
RUN set -ex \
    && export DOCKERIZE_VERSION="2.2.0" \
    && curl -sL -o dockerize.tar.gz "https://github.com/bitpoke/dockerize/releases/download/v${DOCKERIZE_VERSION}/dockerize-linux-amd64-v${DOCKERIZE_VERSION}.tar.gz" \
    && tar -C /usr/local/bin -xzvf dockerize.tar.gz \
    && rm dockerize.tar.gz \
    && chmod 0755 /usr/local/bin/dockerize \
    && chown root:root /usr/local/bin/dockerize

# ------------------------------------------------------------------------------------------------
# sops
RUN set -ex \
    && export SOPS_VERSION="3.7.3" \
    && curl -sL -o /usr/local/bin/sops "https://github.com/mozilla/sops/releases/download/v${SOPS_VERSION}/sops-v${SOPS_VERSION}.linux" \
    && chmod 0755 /usr/local/bin/sops \
    && chown root:root /usr/local/bin/sops

# ------------------------------------------------------------------------------------------------
# helm
RUN set -ex \
    && export HELM_VERSION="3.12.0" \
    && curl -sL -o helm.tar.gz "https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz" \
    && tar -C /usr/local/bin -xzvf helm.tar.gz --strip-components 1 linux-amd64/helm \
    && rm helm.tar.gz \
    && chmod 0755 /usr/local/bin/helm \
    && chown root:root /usr/local/bin/helm

# ------------------------------------------------------------------------------------------------
# helm secrets plugin
RUN set -ex \
    && export HELM_SECRETS_VERSION="3.15.0" \
    && helm plugin install https://github.com/jkroepke/helm-secrets --version ${HELM_SECRETS_VERSION}

# ------------------------------------------------------------------------------------------------
# kustomize
RUN set -ex \
    && export KUSTOMIZE_VERSION="4.5.7" \
    && curl -sL -o kustomize.tar.gz "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_amd64.tar.gz" \
    && tar -C /usr/local/bin -xzvf kustomize.tar.gz \
    && rm kustomize.tar.gz \
    && chmod 0755 /usr/local/bin/kustomize \
    && chown root:root /usr/local/bin/kustomize

# ------------------------------------------------------------------------------------------------
# hugo
RUN set -ex \
    && export HUGO_VERSION="0.111.3" \
    && curl -sLo- "https://github.com/gohugoio/hugo/releases/download/v${HUGO_VERSION}/hugo_extended_${HUGO_VERSION}_Linux-64bit.tar.gz" | tar -zx -C /usr/local/bin hugo \
    && pip3 install pygments \
    && npm install -g postcss postcss-cli

# ------------------------------------------------------------------------------------------------
# firebase-cli
RUN set -ex \
    && npm install -g firebase-tools

# ------------------------------------------------------------------------------------------------
# Run tini as PID 1 and avoid signal handling issues
RUN set -ex \
    && export TINI_VERSION=v0.19.0 \
    && curl -sL -o /tini https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini-${ARCH} \
    && chmod +x /tini

COPY root /

ENTRYPOINT [ "/tini", "-g", "--", "/build/run.sh" ]
