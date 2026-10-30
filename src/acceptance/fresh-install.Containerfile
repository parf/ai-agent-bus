ARG BASE=docker.io/library/archlinux:latest
FROM ${BASE}
# The web face runs on the system bun, which the unit executes at /usr/bin/bun.
# tmux is for the user-account runner acceptance (RT31). bubblewrap is for the upgrade gate's old release: before 0.8.50 the Go
# dashboard ran under it, so a host upgrading from one has it.
RUN if command -v pacman >/dev/null; then \
      pacman -Syu --noconfirm sudo unzip curl bubblewrap python tmux \
      && pacman -Scc --noconfirm; \
    else \
      apt-get update \
      && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
         systemd systemd-sysv sudo unzip curl ca-certificates bubblewrap python3 tmux \
         procps util-linux findutils passwd logrotate openssh-client \
      && rm -rf /var/lib/apt/lists/*; \
    fi \
 && curl -fsSL https://bun.sh/install | BUN_INSTALL=/opt/bun bash \
 && install -m 0755 /opt/bun/bin/bun /usr/bin/bun
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
