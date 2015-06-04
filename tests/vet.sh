#!/bin/bash

for d in `find .. -type d \
            -not -path ".." \
            -not -path "../bin*" \
            -not -path "../root*" \
            -not -path "../.git*" \
            -not -path "../_vendor*" \
            -not -path "../Godeps*" \
            -not -path "../tests*" \
            -not -path "../test-results*"`
do
  echo "Running go vet $d"
  go tool vet --composites=false $d
done

echo "Running go vet ../libcmd.go"
go tool vet --composites=false ../libcmd.go
