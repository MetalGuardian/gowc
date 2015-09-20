FROM ubuntu:latest

MAINTAINER Ivan Pushkin <imetalguardi+docker@gmail.com>

ENV MYSQL_USER root
ENV MYSQL_PASSWORD ""
ENV MYSQL_MAJOR 5.6

RUN apt-key adv --keyserver pool.sks-keyservers.net --recv-keys A4A9406876FCBD3C456770C88C718D3B5072E1F5
RUN echo "deb http://repo.mysql.com/apt/ubuntu/ $(lsb_release -cs) mysql-${MYSQL_MAJOR}" > /etc/apt/sources.list.d/mysql.list

RUN apt-get update

# install all packages
RUN DEBIAN_FRONTEND=noninteractive apt-get -y -q --no-install-recommends install \
    mysql-server \
    mysql-client \
    ca-certificates \
    curl

EXPOSE 8080

COPY ./docker-entrypoint.sh /

RUN mkdir /go

COPY ./gowc /go/

COPY ./dump.sql /go/

ENV PATH /go:$PATH
WORKDIR /go

ENTRYPOINT ["/docker-entrypoint.sh"]

CMD ["gowc"]
