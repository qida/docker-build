FROM golang:latest
LABEL maintainer="qida"
LABEL version="0.0.3"
ARG THRIFT_VERSION
ENV _THRIFT_VERSION=${THRIFT_VERSION}

# RUN sed -i s/deb.debian.org/mirrors.aliyun.com/g /etc/apt/sources.list 
RUN apt-get update
RUN apt-get install libboost-dev libboost-test-dev \
    libboost-program-options-dev libboost-system-dev \
    libboost-filesystem-dev libevent-dev automake \
    libtool flex bison pkg-config libssl-dev wget pip \
    autoconf g++ libboost-all-dev python3-pip python3-setuptools \
    -y
RUN if [ -z "$_THRIFT_VERSION" ] || [ "$_THRIFT_VERSION" = "latest" ]; then \
    _THRIFT_VERSION=$(curl -s https://api.github.com/repos/apache/thrift/releases/latest | grep '"tag_name"' | sed -E 's/.*"tag_name": "v?([0-9.]+)".*/\1/');\
    echo "Using latest Thrift version: $_THRIFT_VERSION";\
    wget -c --show-progress --retry-connrefused --waitretry=3 --timeout=30 https://mirrors.aliyun.com/apache/thrift/${_THRIFT_VERSION}/thrift-${_THRIFT_VERSION}.tar.gz || \
    wget -c --show-progress --retry-connrefused --waitretry=3 --timeout=30 https://downloads.apache.org/thrift/${_THRIFT_VERSION}/thrift-${_THRIFT_VERSION}.tar.gz || \
    wget -c --show-progress --retry-connrefused --waitretry=3 --timeout=30 https://archive.apache.org/dist/thrift/${_THRIFT_VERSION}/thrift-${_THRIFT_VERSION}.tar.gz && \
    mkdir -p thrift && tar -xzf thrift-${_THRIFT_VERSION}.tar.gz --strip-components=1 -C thrift; \
    fi
RUN cd thrift && ./configure
RUN cd thrift && make && make install