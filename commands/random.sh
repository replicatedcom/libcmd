#!/bin/bash

< /dev/urandom tr -dc ${2:-_A-Z-a-z-0-9} | head -c${1:-16};echo;