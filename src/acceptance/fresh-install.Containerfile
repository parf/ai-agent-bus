FROM docker.io/library/archlinux:latest
# The web face runs on the system bun, which the unit executes at /usr/bin/bun.
RUN pacman -Syu --noconfirm sudo unzip curl \
 && curl -fsSL https://bun.sh/install | BUN_INSTALL=/opt/bun bash \
 && install -m 0755 /opt/bun/bin/bun /usr/bin/bun \
 && pacman -Scc --noconfirm
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
