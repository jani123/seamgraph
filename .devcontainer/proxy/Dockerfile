FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends squid python3 ca-certificates \
    && rm -rf /var/lib/apt/lists/*
# Policy is copied into the image, never mounted from the writable checkout.
COPY squid.conf allowed-domains.txt /etc/squid/
RUN squid -k parse -f /etc/squid/squid.conf
USER proxy
ENTRYPOINT ["squid", "-N", "-f", "/etc/squid/squid.conf"]
