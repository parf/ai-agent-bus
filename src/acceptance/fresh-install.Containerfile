FROM docker.io/library/archlinux:latest
RUN pacman -Syu --noconfirm bubblewrap chromium python-playwright sudo \
 && pacman -Scc --noconfirm
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
