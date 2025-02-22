# Copyright (C) The Arvados Authors. All rights reserved.
#
# SPDX-License-Identifier: AGPL-3.0

ARG HOSTTYPE
ARG BRANCH
ARG GOVERSION

FROM rockylinux:8.8-minimal as build_x86_64
ONBUILD ARG BRANCH
# Install go
ONBUILD ARG GOVERSION
ONBUILD ADD generated/go${GOVERSION}.linux-amd64.tar.gz /usr/local/
ONBUILD RUN ln -s /usr/local/go/bin/go /usr/local/bin/
# Install nodejs and npm
ONBUILD ADD generated/node-v12.22.12-linux-x64.tar.xz /usr/local/
ONBUILD RUN ln -s /usr/local/node-v12.22.12-linux-x64/bin/* /usr/local/bin/
ONBUILD RUN npm install -g yarn
ONBUILD RUN ln -sf /usr/local/node-v12.22.12-linux-x64/bin/* /usr/local/bin/

FROM rockylinux:8.8-minimal as build_aarch64
ONBUILD ARG BRANCH
# Install go
ONBUILD ARG GOVERSION
ONBUILD ADD generated/go${GOVERSION}.linux-arm64.tar.gz /usr/local/
ONBUILD RUN ln -s /usr/local/go/bin/go /usr/local/bin/
# Install nodejs and npm
ONBUILD ADD generated/node-v12.22.12-linux-arm64.tar.xz /usr/local/
ONBUILD RUN ln -s /usr/local/node-v12.22.12-linux-arm64/bin/* /usr/local/bin/
ONBUILD RUN npm install -g yarn
ONBUILD RUN ln -sf /usr/local/node-v12.22.12-linux-arm64/bin/* /usr/local/bin/

FROM build_${HOSTTYPE}

MAINTAINER Arvados Package Maintainers <packaging@arvados.org>

# Install dependencies.
RUN microdnf --assumeyes --enablerepo=devel install \
    automake \
    bison \
    bzip2 \
    fuse-devel \
    gcc \
    gcc-c++ \
    git \
    libattr-devel \
    libcurl-devel \
    libffi-devel \
    libtool \
    libyaml-devel \
    make \
    nss-devel \
    openssl-devel \
    pam-devel \
    patch \
    postgresql-devel \
    procps-ng \
    python39 \
    python39-devel \
    readline-devel \
    rpm-build \
    ruby \
    sqlite-devel \
    tar \
    unzip \
    wget \
    which \
    xz-libs \
    zlib-devel

# Install RVM
ADD generated/mpapis.asc /tmp/
ADD generated/pkuczynski.asc /tmp/
RUN gpg --import --no-tty /tmp/mpapis.asc && \
    gpg --import --no-tty /tmp/pkuczynski.asc && \
    curl -L https://get.rvm.io | bash -s stable && \
    /usr/local/rvm/bin/rvm install --disable-binary 2.7 -j $(grep -c processor /proc/cpuinfo) && \
    /usr/local/rvm/bin/rvm alias create default ruby-2.7 && \
    echo "gem: --no-document" >> ~/.gemrc && \
    /usr/local/rvm/bin/rvm-exec default gem install bundler --version 2.2.19 && \
    /usr/local/rvm/bin/rvm-exec default gem install fpm --version 1.15.1

RUN /usr/local/rvm/bin/rvm-exec default bundle config --global jobs $(let a=$(grep -c processor /proc/cpuinfo )-1; echo $a)
# Cf. https://build.betterup.com/one-weird-trick-that-will-speed-up-your-bundle-install/
ENV MAKE "make --jobs $(grep -c processor /proc/cpuinfo)"

# Preseed the go module cache and the ruby gems, using the currently checked
# out branch of the source tree. This avoids potential compatibility issues
# between the version of Ruby and certain gems.
RUN git clone git://git.arvados.org/arvados.git /tmp/arvados && \
    cd /tmp/arvados && \
    if [[ -n "${BRANCH}" ]]; then git checkout ${BRANCH}; fi && \
    cd /tmp/arvados/services/api && \
    /usr/local/rvm/bin/rvm-exec default bundle install && \
    cd /tmp/arvados && \
    go mod download

ENV WORKSPACE /arvados
CMD ["/usr/local/rvm/bin/rvm-exec", "default", "bash", "/jenkins/run-build-packages.sh", "--target", "rocky8"]
