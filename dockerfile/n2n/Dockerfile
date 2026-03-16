FROM centos:8 AS builder
LABEL email="sunqida@foxmail.com"
RUN sed -i 's/mirrorlist/#mirrorlist/g' /etc/yum.repos.d/CentOS-* && \
    sed -i 's|#baseurl=http://mirror.centos.org|baseurl=http://vault.centos.org|g' /etc/yum.repos.d/CentOS-*
RUN yum update -y
RUN yum groupinstall "Development Tools"  -y
RUN yum install openssl-devel svn net-tools -y
WORKDIR /n2n
# RUN git clone --depth=1 https://github.com/ntop/n2n.git
RUN export LIBS= && export CFLAGS=
COPY . .
RUN chmod +x autogen.sh && ./autogen.sh
RUN ./configure
RUN make && make install

FROM alpine:latest
WORKDIR /usr/sbin
#设置代理镜像
RUN alpine_version=`cat /etc/issue | head -1 | awk '{print $5}'` \
   && echo -e https://mirrors.ustc.edu.cn/alpine/v${alpine_version}/main/ > /etc/apk/repositories 
# RUN 设置 Asia/Shanghai 时区
RUN apk --update --no-cache add tzdata libc6-compat \
    && ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone 
#RUN mkdir /dev/net && mknod /dev/net/tun c 10 200
# COPY 源路径 目标路径 从镜像中 COPY
COPY --from=builder /usr/sbin/supernode /usr/sbin/
COPY --from=builder /usr/sbin/edge /usr/sbin/
RUN chmod +x /usr/sbin/supernode && chmod +x /usr/sbin/edge
# EXPOSE 设置端口映射
EXPOSE 7654/udp
# 由于alpine镜像使用的是musl libc而不是gnu libc，/lib64/ 是不存在的
#RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
CMD ["ls","-al"]