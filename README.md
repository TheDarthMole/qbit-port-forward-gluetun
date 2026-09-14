# qBittorrent Port Forwarding for Gluetun

<p align="center">
  <img src="https://raw.githubusercontent.com/qdm12/gluetun/refs/heads/master/doc/logo.svg" alt="Gluetun Logo" width="150">
  <br>
  <strong>A robust Go utility to automatically sync Gluetun's forwarded port with qBittorrent.</strong>
</p>

This utility, running in a lightweight Docker container, periodically checks the VPN port forwarded by **Gluetun** and updates **qBittorrent's Listening Port** accordingly. This ensures your torrent client is always configured for optimal performance behind the VPN.

This project is a Go rewrite of the original shell script by [@mjmeli](https://github.com/mjmeli/qbittorrent-port-forward-gluetun-server), offering improved logging, error handling, and stability.

---

## 🔧 Configuration

### Environment Variables

The container is configured using the following environment variables:

| Variable | Description | Default | Required |
| :--- | :--- | :--- | :--- |
| `QBT_API_KEY` | Your qBittorrent WebAPI key. | None | **Yes** |
| `QBT_ADDR` | The full HTTP or HTTPS URL for the qBittorrent WebUI. | `http://localhost:8080` | No |
| `GTN_ADDR` | The full HTTP or HTTPS URL for the Gluetun control server. | `http://localhost:8000` | No |

qBittorrent API-key authentication requires qBittorrent `>= 5.2.0` or WebAPI `>= 2.14.1`.
Generate the key in qBittorrent under **Preferences -> WebUI -> API Key**.
This utility sends the key as an `Authorization: Bearer <key>` header and does not call qBittorrent's auth endpoints.

### HTTPS and Private Certificate Authorities

The image includes Alpine's public CA bundle. For an endpoint signed by your own
CA, mount its PEM certificate into the trust directory:

```yaml
volumes:
  - ./private-ca.crt:/etc/ssl/certs/private-ca.crt:ro
```

The certificate must be readable by the container's user (`65532:65532`). This
adds your CA alongside the public roots. HTTPS certificate and hostname
verification remain enabled.

---

## 🚀 Example with Docker Compose

This is an example of how to integrate this utility with `gluetun` and `qbittorrent` services in a `docker-compose.yml` file.

### Gluetun Control Server Setup

For this script to read the forwarded port, you must enable Gluetun's HTTP control server and give this utility permission to access the port information.

1.  **Enable Control Server:** You must set the `HTTP_CONTROL_SERVER_ADDRESS` environment variable in your `gluetun` service.
2.  **Create Auth Config:** The control server needs a `config.toml` file to define access rules. Create this file in a directory on your host that you will mount into the container (e.g., `./gluetun-data/auth/config.toml`).

    **`config.toml` content:**
```
[[roles]]
name = "port-forward"
# Allow access to the port forwarding endpoint
routes = ["GET /v1/portforward"]
auth = "none"
```

### Docker-Compose Example

```yaml
version: "3.7"

services:
  gluetun:
    image: qmcgaw/gluetun:latest
    container_name: gluetun
    restart: always
    ports:
      - "8112:8112" # exposing the qbt webui
    cap_add:
      - NET_ADMIN
    devices:
      - /dev/net/tun:/dev/net/tun
    volumes:
      # Mount the directory containing config.toml to /gluetun/auth
      - ./path/to/your/auth/folder:/gluetun/auth
    environment:
      # VPN Configuration (replace with your provider)
      - VPN_SERVICE_PROVIDER=protonvpn
      - OPENVPN_USER=xxxxxxxx+pmp
      - OPENVPN_PASSWORD=xxxxxxxx
      - SERVER_COUNTRIES=Netherlands
      # Port Forwarding
      - VPN_PORT_FORWARDING=on
      - PORT_FORWARD_ONLY=on
      # Enable the control server so the port-forward script can read the port
      - HTTP_CONTROL_SERVER_ADDRESS=:8000
    networks:
      - arr_network

  qbittorrent:
    image: lscr.io/linuxserver/qbittorrent:latest
    restart: always
    # This forces all of qbittorrent's traffic through the gluetun container
    network_mode: service:gluetun
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Europe/Berlin
      - WEBUI_PORT=8112
    depends_on:
      - gluetun

  port-forward:
    image: kirari04/qbittorrent-port-forward-gluetun-server:latest
    restart: always
    networks:
      - arr_network
    environment:
      # Use the service name and internal port for qBittorrent
      - QBT_ADDR=http://gluetun:8112
      # Use the service name for the Gluetun control server
      - GTN_ADDR=http://gluetun:8000
      # Required: qBittorrent API key generated in Preferences -> WebUI -> API Key
      - QBT_API_KEY=qbt_XXXXXXXXXXXXXXXXXXXXXXXXXXXX
    depends_on:
      - gluetun
      - qbittorrent

networks:
  arr_network:
    driver: bridge
```

---

## 🛠️ Development

If you wish to build the image yourself.

### Build the Docker Image

```bash
docker build . -t kirari04/qbittorrent-port-forward-gluetun-server:latest
```

The runtime uses `scratch` and contains a static Go binary and a CA bundle copied
from the Go Alpine builder. It runs as user `65532:65532` and supports a read-only
root filesystem. It has no shell or package manager.

### Run the Container Tests

```bash
docker build --target container-test -t port-forward:container-test .
docker run --rm --network none --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,nodev,mode=1777 \
  --cap-drop ALL --security-opt no-new-privileges \
  port-forward:container-test
```

These tests launch the application inside its runtime filesystem, using local
mock APIs. They cover HTTP and HTTPS port updates, private CA trust, rejection of
untrusted certificates, startup configuration, and already-correct ports. The
test executable is only included in the `container-test` target. The temporary
filesystem is used for test certificates; the application needs no writable
storage. CI runs these tests before publishing on a push to `main`.

### Maintain the Image

The builder is pinned by version and digest so compiler and CA changes are
reviewable. Dependabot checks the Docker image daily and Go modules and GitHub
Actions weekly. Review and merge those update PRs to publish refreshed images,
then pull the new image and recreate running containers. Rebuilding an unchanged
digest does not update its CA bundle.

The `go` directive in `go.mod` records the minimum supported Go version. The
Dockerfile selects the current compiler used for container releases.

### Run the Container Manually

```bash
docker run --rm -it \
  -e QBT_API_KEY=qbt_XXXXXXXXXXXXXXXXXXXXXXXXXXXX \
  -e QBT_ADDR=http://192.168.1.100:8080 \
  -e GTN_ADDR=http://192.168.1.100:8000 \
  kirari04/qbittorrent-port-forward-gluetun-server:latest
```
