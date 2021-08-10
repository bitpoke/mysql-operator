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

FROM google/cloud-sdk:290.0.0

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
    && rm -f /etc/apt/sources.list.d/php.list

# ------------------------------------------------------------------------------------------------
# git config
RUN git config --global user.email "bot@presslabs.com" \
    && git config --global user.name "Igor Debot"

# ------------------------------------------------------------------------------------------------
# Go support
RUN GO_VERSION=1.13.10 && \
    GO_HASH=8a4cbc9f2b95d114c38f6cbe94a45372d48c604b707db2057c787398dfbf8e7f && \
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
RUN curl -fsSL https://deb.nodesource.com/setup_10.x | bash - && \
    curl -fsSL https://dl-ssl.google.com/linux/linux_signing_key.pub | apt-key add - && \
    echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" > /etc/apt/sources.list.d/google.list && \
    /usr/local/build-scripts/install-packages.sh \
        nodejs \
        google-chrome-stable \
        xvfb && \
    rm -f /etc/apt/sources.list.d/google.list && \
    ln -fs /usr/local/bin/xvfb-chrome /usr/bin/google-chrome
ENV CHROME_BIN /usr/bin/google-chrome

# ------------------------------------------------------------------------------------------------
# Yarn
RUN curl -sS https://dl.yarnpkg.com/debian/pubkey.gpg | sudo apt-key add - && \
    echo "deb https://dl.yarnpkg.com/debian/ stable main" | sudo tee /etc/apt/sources.list.d/yarn.list && \
    /usr/local/build-scripts/install-packages.sh \
        yarn && \
    rm -f /etc/apt/sources.list.d/yarn.list

# ------------------------------------------------------------------------------------------------
# rclone
RUN set -ex \
    && export RCLONE_VERSION=1.50.2 \
    && curl -sL -o rclone-v${RCLONE_VERSION}-linux-amd64.deb https://github.com/rclone/rclone/releases/download/v${RCLONE_VERSION}/rclone-v${RCLONE_VERSION}-linux-amd64.deb \
    && dpkg -i rclone-v${RCLONE_VERSION}-linux-amd64.deb \
    && rm rclone-v${RCLONE_VERSION}-linux-amd64.deb

# ------------------------------------------------------------------------------------------------
# Run tini as PID 1 and avoid signal handling issues
ARG ARCH
ARG TINI_VERSION
RUN curl -sL -o /tini https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini-${ARCH} && \
    chmod +x /tini

ENV DOCKER_HOST=unix:///workspace/docker.sock

COPY root /

ENTRYPOINT [ "/tini", "-g", "--", "/build/run.sh" ]
