#!/bin/bash

# generate private key
openssl genrsa -des3 -out $HOME/server.key.orig -passout pass:temp ${1:-1024} > /dev/null 2>&1
# generate csr
openssl req -new -key $HOME/server.key.orig -out $HOME/server.csr -passin pass:temp -subj "/C=US/ST=California/L=Los Angeles/O=Replicated/CN=example.com" > /dev/null 2>&1
# remove passphrase from key
openssl rsa -in $HOME/server.key.orig -out $HOME/server.key -passin pass:temp > /dev/null 2>&1
# print key
cat $HOME/server.key
# generate self signed cert
openssl x509 -req -days 365 -in $HOME/server.csr -signkey $HOME/server.key -out $HOME/server.crt > /dev/null 2>&1
# print cert
cat $HOME/server.crt