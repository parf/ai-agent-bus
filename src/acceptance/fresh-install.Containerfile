FROM docker.io/library/archlinux:latest
RUN pacman -Sy --noconfirm bubblewrap sudo \
 && pacman -Scc --noconfirm
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
