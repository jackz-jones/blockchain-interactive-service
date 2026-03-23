#!/bin/bash

SERVER_NAME=$1

rm -rf ./pb
goctl rpc protoc --go_out=./pb --go-grpc_out=./pb --zrpc_out=. ./proto/${SERVER_NAME}.proto >> /dev/null
protoc --go_out=./pb --go-grpc_out=./pb ./proto/*.proto
rm -rf ./scripts/internal

go fmt ./*.go >> /dev/null
#go fmt ./internal/logic/* >> /dev/null
go fmt ./internal/server/* >> /dev/null
