#!/bin/bash

pushd ..
go vet ./...
if [ $? -ne 0 ]; then
  popd
  exit 1
fi

godep go test -cover \
  -coverpkg github.com/replicatedcom/libcmd/... \
  -v \
  ./...
if [ $? -ne 0 ]; then
  popd
  exit 1
fi

popd
