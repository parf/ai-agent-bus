# A clean Linux systemd host for the checkout install in src/INSTALL.md: git, a C compiler and Python, nothing of
# the bus. Go and Bun are the host's, mounted read-only at run time, because the container has no network.
ARG BASE=docker.io/library/debian:13
FROM ${BASE}
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    systemd systemd-sysv sudo git gcc libc6-dev python3 unzip curl ca-certificates openssh-client \
    procps util-linux findutils passwd \
    && rm -rf /var/lib/apt/lists/*
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
