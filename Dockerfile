FROM ubuntu:trusty

# Install.
RUN apt-get update && apt-get install -y \
    openssl \
    curl

ADD ./root/commands /root/commands

# Set environment variables.
ENV HOME /root

# Define working directory.
WORKDIR /root

# Define default command.
CMD ["bash"]