"""Exercise the running proxy and check that direct egress stays unavailable."""

import os
import socket
import struct
import urllib.error
import urllib.request


def expect_proxy_response(url, allowed):
    request = urllib.request.Request(url, method="HEAD")
    try:
        response = urllib.request.urlopen(request, timeout=20)
    except urllib.error.HTTPError as error:
        response = error
    except urllib.error.URLError as error:
        # HTTPS denial happens while establishing the CONNECT tunnel.
        if not allowed and "Tunnel connection failed: 403" in str(error.reason):
            print(f"Blocked through proxy: {url}")
            return
        raise AssertionError(f"Unexpected connection failure for {url}: {error}") from error
    with response:
        denied = response.status == 403 and response.headers.get("X-Squid-Error")
        if allowed and denied:
            raise AssertionError(f"Allowlisted destination rejected: {url}")
        if not allowed and not denied:
            raise AssertionError(f"Disallowed destination was reachable: {url}")
    print(f"{'Allowed' if allowed else 'Blocked'} through proxy: {url}")


def expect_no_direct_connection(host, port):
    try:
        with socket.create_connection((host, port), timeout=2):
            pass
    except OSError:
        print(f"Direct connection blocked: {host}:{port}")
        return
    raise AssertionError(f"Direct connection succeeded: {host}:{port}")


def main():
    assert os.environ.get("HTTPS_PROXY") == "http://proxy:3128", "Proxy environment missing"
    for url in (
        "https://go.dev/doc/",
        "https://pkg.go.dev/",
        "https://github.com/",
        "https://proxy.golang.org/golang.org/x/tools/@v/list",
        "https://cache.nixos.org/nix-cache-info",
        "https://registry.npmjs.org/typescript",
        "https://pypi.org/simple/pytest/",
        "https://chatgpt.com/",
        "https://auth.openai.com/",
    ):
        expect_proxy_response(url, allowed=True)
    for url in (
        "https://example.com/",
        "http://example.com/",
        "https://github.com.evil.example/",
        "https://1.1.1.1/",
        "http://169.254.169.254/latest/meta-data/",
        "https://go.dev:8443/",
    ):
        expect_proxy_response(url, allowed=False)

    # Raw sockets ignore all proxy environment variables.
    for host in ("1.1.1.1", "8.8.8.8", "172.17.0.1", "192.168.1.1", "2606:4700:4700::1111"):
        expect_no_direct_connection(host, 443)
    # Root must not be able to reconfigure networking or spoof traffic.
    status = dict(line.split(":", 1) for line in open("/proc/self/status") if ":" in line)
    bounding = int(status["CapBnd"].strip(), 16)
    assert not bounding & ((1 << 12) | (1 << 13)), "NET_ADMIN/NET_RAW must be dropped"
    routes = open("/proc/net/route").read().splitlines()[1:]
    assert not any(line.split()[1] == "00000000" for line in routes), "Unexpected default IPv4 route"

    # Docker service discovery still works, but public DNS must not become
    # an alternate path for sending data outside the isolated network.
    socket.getaddrinfo("proxy", 3128)
    query = struct.pack("!6H", 1234, 256, 1, 0, 0, 0)
    query += b"\x07example\x03com\x00" + struct.pack("!HH", 1, 1)
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as dns:
        dns.settimeout(2)
        dns.sendto(query, ("127.0.0.11", 53))
        try:
            result = dns.recv(4096)
        except socket.timeout:
            result = None
        if result is not None:
            assert struct.unpack("!6H", result[:12])[3] == 0, "External Docker DNS forwarding is enabled"
    print("Network isolation checks passed")


if __name__ == "__main__":
    main()
