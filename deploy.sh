#!/bin/bash

go clean
go build -ldflags "-w -s"

target=/var/hiprice/hiprice-dispatcher/

mkdir -p $target
cp -f hiprice-dispatcher $target
cp -f conf.yaml $target

cd $target
nohup ./hiprice-dispatcher > /dev/null 2>&1 &
