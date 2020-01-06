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

FROM ubuntu:16.04

ENV DEBIAN_FRONTEND noninteractive

COPY build-scripts /usr/local/build-scripts
# ------------------------------------------------------------------------------------------------
# install build and release tools
RUN /usr/local/build-scripts/install-packages.sh \
        apt-transport-https \
        gnupg-agent \
        software-properties-common \
        python \
        gettext \
        build-essential \
        ca-certificates \
        curl \
        git \
        jq \
        lsb-release \
        make \
        openssh-client \
        rsync \
        runit \
        sudo \
        zip

# ------------------------------------------------------------------------------------------------
# install docker
RUN curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add - \
    && add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
    && /usr/local/build-scripts/install-packages.sh docker-ce docker-ce-cli containerd.io

# ------------------------------------------------------------------------------------------------
# Install pip & AWSCLI
RUN curl https://bootstrap.pypa.io/get-pip.py -o get-pip.py && \
    python get-pip.py && \
    pip install awscli --upgrade

# ------------------------------------------------------------------------------------------------
# Install google cloud sdk
# https://cloud.google.com/sdk/docs/downloads-versioned-archives
ENV GCLOUD_SDK_VERSION="254.0.0"
ENV CLOUDSDK_PYTHON="/usr/bin/python2.7"
ENV GOOGLE_APPLICATION_CREDENTIALS="/run/google-credentials.json"
RUN curl -sL -o google-cloud-sdk.tar.gz https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-${GCLOUD_SDK_VERSION}-linux-x86_64.tar.gz \
    && tar -zxf google-cloud-sdk.tar.gz \
    && mv google-cloud-sdk /opt/ \
    && rm google-cloud-sdk.tar.gz \
    && /opt/google-cloud-sdk/bin/gcloud --quiet components install beta
ENV PATH /opt/google-cloud-sdk/bin:$PATH

# ------------------------------------------------------------------------------------------------
# Go support
RUN GO_VERSION=1.13.4 && \
    GO_HASH=692d17071736f74be04a72a06dab9cac1cd759377bd85316e52b2227604c004c && \
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
    rm -f /etc/apt/sources.list.d/google.list

# setup chrome to run inside the container
ADD xvfb-chrome /usr/bin/xvfb-chrome
RUN ln -fs /usr/bin/xvfb-chrome /usr/bin/google-chrome
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
# git config
RUN git config --global user.email "bot@presslabs.com" \
    && git config --global user.name "Igor Debot"

# ------------------------------------------------------------------------------------------------
# Run tini as PID 1 and avoid signal handling issues
ARG ARCH
ARG TINI_VERSION
ADD https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini-${ARCH} /tini
RUN chmod +x /tini

COPY setup-credentials-helper.sh /usr/local/bin/setup-credentials-helper.sh
COPY run.sh rsyncd.sh /build/

ENTRYPOINT [ "/tini", "-g", "--", "/build/run.sh" ]
