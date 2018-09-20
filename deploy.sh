#!/bin/sh

go build -ldflags "-w -s"
mkdir -p /hiprice
cp -f hiprice-dispatcher /hiprice/dispatcher
cp -f conf.yaml /hiprice/
go clean

cd /hiprice
nohup ./dispatcher > /dev/null 2>&1 &