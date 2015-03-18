#!/bin/bash

# remove file extension
COMMAND=$(echo $1 | cut -d'.' --complement -f2-)
# remove first argument from positional arguments
shift
# run command in container
sudo docker run freighter/cmd bash /root/commands/${COMMAND}.sh $@