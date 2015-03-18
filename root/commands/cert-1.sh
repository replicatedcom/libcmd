#!/bin/bash

# generate key and cert
openssl req \
  -new \
  -newkey rsa:${1:-1024} \
  -days 365 \
  -nodes \
  -x509 \
  -subj "/C=US/ST=California/L=Los Angeles/O=Replicated/CN=example.com" \
  -keyout $HOME/server.key \
  -out $HOME/server.crt > /dev/null 2>&1
# print files to stdout
cat $HOME/server.key
cat $HOME/server.crt