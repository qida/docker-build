FROM golang:latest
LABEL maintainer "qida"
LABEL version "0.0.2"
ARG THRIFT_VERSION=latest
ENV THRIFT_VERSION=${THRIFT_VERSION:-latest}
RUN if [ -z "$THRIFT_VERSION" ] || [ "$THRIFT_VERSION" = "latest" ]; then \
        THRIFT_VERSION=$(curl -s https://api.github.com/repos/apache/thrift/releases/latest | grep -o '"tag_name":"v[^"]*"' | head -1 | cut -d'"' -f4 | sed 's/^v//') && \
        echo "Using latest Thrift version: $THRIFT_VERSION"; \
    fi
RUN sed -i s/deb.debian.org/mirrors.aliyun.com/g /etc/apt/sources.list 
RUN apt-get update
RUN apt-get install libboost-dev libboost-test-dev \
    libboost-program-options-dev libboost-system-dev \
    libboost-filesystem-dev libevent-dev automake \
    libtool flex bison pkg-config libssl-dev wget pip \
    -y
RUN wget http://archive.apache.org/dist/thrift/${THRIFT_VERSION}/thrift-${THRIFT_VERSION}.tar.gz
RUN tar -xvf thrift-${THRIFT_VERSION}.tar.gz
RUN cd thrift-${THRIFT_VERSION} && ./configure
RUN cd thrift-${THRIFT_VERSION} && make && make install