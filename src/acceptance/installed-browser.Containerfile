# The fresh-install host plus a real browser. The web face loads its fonts,
# icons and charts from a pinned CDN, so this gate's container has a network.
FROM localhost/agent-bus-fresh-install:arch-systemd
RUN pacman -Syu --noconfirm chromium python-playwright \
 && pacman -Scc --noconfirm
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
