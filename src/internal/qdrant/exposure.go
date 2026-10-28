package qdrant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

func validateListeners(data []byte, pid, port string) error {
	seen := false
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, "pid="+pid+",") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return errors.New("cannot parse Qdrant listener")
		}
		if fields[3] != "127.0.0.1:"+port {
			return fmt.Errorf("Qdrant has an unexpected listener %s", fields[3])
		}
		seen = true
	}
	if !seen {
		return errors.New("Qdrant's exact loopback listener was not observed")
	}
	return nil
}
func (h *Host) exposure(ctx context.Context, container string, port int) error {
	out, err := h.command(ctx, "podman", "container", "inspect", "--format", "{{.State.Pid}}", container)
	if err != nil {
		return err
	}
	pid := strings.TrimSpace(string(out))
	if n, e := strconv.Atoi(pid); e != nil || n <= 0 {
		return errors.New("Qdrant PID unavailable")
	}
	sockets, err := h.command(ctx, "ss", "-H", "-ltnp")
	if err != nil {
		return err
	}
	if err := validateListeners(sockets, pid, strconv.Itoa(port)); err != nil {
		return err
	}
	route, err := h.command(ctx, "sysctl", "-n", "net.ipv4.conf.all.route_localnet")
	if err != nil {
		return err
	}
	value := strings.TrimSpace(string(route))
	if value != "0" && value != "1" {
		return errors.New("invalid route_localnet observation")
	}
	h.say("route_localnet=" + value + " (not changed)")
	data, err := h.command(ctx, "ip", "-json", "address", "show")
	if err != nil {
		return err
	}
	var interfaces []struct {
		Ifname string `json:"ifname"`
		Addr   []struct {
			Local string `json:"local"`
		} `json:"addr_info"`
	}
	if json.Unmarshal(data, &interfaces) != nil {
		return errors.New("invalid host address observation")
	}
	checked := 0
	for _, in := range interfaces {
		for _, a := range in.Addr {
			ip := net.ParseIP(a.Local)
			if ip == nil {
				return errors.New("invalid host IP address")
			}
			if ip.IsLoopback() || ip.IsUnspecified() {
				continue
			}
			host := a.Local
			if ip.IsLinkLocalUnicast() && ip.To4() == nil {
				host += "%" + in.Ifname
			}
			conn, e := h.Dial(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
			if e == nil {
				conn.Close()
				return fmt.Errorf("Qdrant reachable on non-loopback address %s", host)
			}
			if ctx.Err() != nil {
				return errors.New("exposure probe deadline exceeded")
			}
			checked++
		}
	}
	raw, err := h.command(ctx, "podman", "container", "inspect", container)
	if err != nil {
		return err
	}
	if h.key != "" && strings.Contains(string(raw), h.key) {
		return errors.New("Qdrant key leaked into container inspection")
	}
	if checked == 0 {
		h.say("WARN no non-loopback addresses: zero external connection probes; listener assertion passed")
		return nil
	}
	h.say(fmt.Sprintf("PASS exact loopback listener, gRPC off; %d non-loopback IPv4/IPv6 connection probes refused; inspect contains no key", checked))
	return nil
}
