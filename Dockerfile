FROM golang:1.11-alpine3.8

LABEL maintainer="kwf2030 <kwf2030@163.com>" \
      version="0.1.0"

RUN echo "http://mirrors.aliyun.com/alpine/v3.8/main" > /etc/apk/repositories && \
    echo "http://mirrors.aliyun.com/alpine/v3.8/community" >> /etc/apk/repositories

RUN apk update && \
    apk upgrade && \
    apk add --no-cache git && \
    mkdir -p $GOPATH/src/golang.org/x $GOPATH/src/go.etcd.io /hiprice

WORKDIR $GOPATH/src/golang.org/x

RUN git clone https://github.com/golang/net.git

WORKDIR $GOPATH/src/go.etcd.io

RUN git clone https://github.com/etcd-io/bbolt

RUN go get github.com/kwf2030/hiprice-dispatcher

WORKDIR $GOPATH/src/github.com/kwf2030/hiprice-dispatcher

RUN go build -ldflags "-w -s" && \
    cp hiprice-dispatcher /hiprice/dispatcher && \
    cp conf.yaml /hiprice/ && \
    go clean

WORKDIR /hiprice

ENTRYPOINT ["./dispatcher"]