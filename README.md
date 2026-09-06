# Raven v2.0
**Modern Network Monitoring for Home and Small Networks**

## Introduction

Raven is a lightweight, modern network monitoring system designed specifically for home networks and small environments. Unlike enterprise monitoring solutions that are bloated with unnecessary features, Raven focuses on what you actually need: **a simple way to see what's working and what's not on your network**.

### Why Raven v2.0?

Home networks today are filled with IoT devices, smart home gadgets, servers, and countless other connected devices. When something goes wrong, you need to quickly identify if it's your router, a switch, or a specific device that's causing issues.

Enterprise monitoring solutions are overkill for home use. They include:
- Complex notification systems
- Multi-level access controls  
- SLA enforcement and documentation
- Performance archiving databases
- Ticketing system integration

**What you actually want:** A clean web interface that quickly shows you if your Xbox can't connect because your router crashed or because the switch it's plugged into is offline.

## Features

### 🚀 **Modern & Fast**
- Built in Go for performance and low resource usage
- Runs perfectly on Raspberry Pi (even Pi Zero W!)
- Real-time WebSocket updates
- Responsive web interface that works on mobile

### 🎯 **Simple & Focused**  
- YAML configuration (no complex setup)
- Automatic network discovery with `raven-discover`
- Built-in ping and service checks
- Nagios plugin compatibility

### 📊 **Monitoring & Metrics**
- Real-time dashboard with host status
- Prometheus metrics integration
- Historical data tracking
- Multiple check types (ping, TCP, DNS, HTTP/HTTPS, SSH, plus Nagios/Icinga external plugins)

### 🔧 **Easy Deployment**
- Debian packages with systemd integration
- Docker support
- Secure by default with proper user isolation
- Professional service management

## Quick Start

### Option 1: Debian Package (Recommended)

```bash
# Build and install
make deb
sudo dpkg -i build/raven_2.0.0_amd64.deb

# Auto-discover your network
sudo raven-discover -network 192.168.1.0/24 -output /tmp/config.yaml
sudo cp /tmp/config.yaml /etc/raven/config.yaml

# Start the service
sudo systemctl start raven

# Access web interface
open http://localhost:8000
```

### Option 2: From Source

```bash
# Build
git clone https://github.com/John-MustangGT/raven2
cd raven2
make build

# Run
./bin/raven -config config.yaml
```

## Network Discovery

Raven v2.0 includes a powerful network discovery tool:

```bash
# Basic network scan
raven-discover -network 192.168.1.0/24

# Advanced options with OS detection
sudo raven-discover \
  -network 10.0.0.0/16 \
  -group "home-network" \
  -dhcp "100-199" \
  -os \
  -output network-config.yaml
```

This automatically generates:
- Host configurations for all discovered devices
- Ping checks for connectivity monitoring  
- Service-specific checks (SSH, HTTP, HTTPS, etc.) based on open ports

## Architecture

### Raven v2.0 vs v1.0

**v1.0 (Legacy):**
- INI configuration files
- Plugin system with .so files
- Basic web templates
- Single-threaded scheduler

**v2.0 (Current):**
- Modern YAML configuration
- Built-in check types with plugin interface
- Vue.js single-page application
- Concurrent job scheduler with workers
- BoltDB embedded database
- Prometheus metrics
- WebSocket real-time updates
- Systemd integration
- Security hardening

### Components

- **Main Daemon** (`raven`): Core monitoring engine
- **Discovery Tool** (`raven-discover`): Network scanning and config generation  
- **Web Interface**: Modern responsive UI with real-time updates
- **Database**: Embedded BoltDB for reliability
- **Metrics**: Prometheus-compatible metrics endpoint

## Configuration

### Basic Configuration (`/etc/raven/config.yaml`)

```yaml
server:
  port: ":8000"
  workers: 3

database:
  path: "/var/lib/raven/data/raven.db"
  history_retention: "720h"  # 30 days

monitoring:
  default_interval: "5m"
  timeout: "30s"

hosts:
  - id: "router"
    name: "router"
    display_name: "Home Router"
    ipv4: "192.168.1.1"
    group: "infrastructure"
    enabled: true

checks:
  - id: "ping-check"
    name: "Connectivity Check"
    type: "ping"
    hosts: ["router"]
    interval:
      ok: "5m"
      warning: "2m"
      critical: "1m"
    enabled: true
```

### Check Types

- **ping**: ICMP connectivity tests
- **tcp**: Raw TCP port connect test
- **dns**: DNS record lookup (A/AAAA/CNAME/MX/TXT/NS), optionally against a specific server, with an optional expected-value match
- **http**: HTTP/HTTPS check (HEAD by default) with TLS certificate validation and expected-status checking
- **ssh**: TCP connect plus SSH identification banner validation
- **nagios** / **icinga**: Runs an external Nagios/Icinga-compatible plugin binary (`options.command` + `options.args`, with `$HOSTADDRESS$`/`$HOSTNAME$` macros)

## Performance

Tested on Raspberry Pi Zero W:
- **18 hosts, 42 active checks**
- **<5% CPU usage**
- **<3% memory usage**
- **Minimal disk I/O** (perfect for SD cards)

## History & Evolution

This project evolved from **Kassandra** (Python prototype) → **Raven v1.0** (Go rewrite) → **Raven v2.0** (modern architecture).

The name comes from Odin's ravens [Huginn and Muninn](https://en.wikipedia.org/wiki/Huginn_and_Muninn) who served as his agents, gathering information from across the nine worlds.

## Documentation

- **Installation**: See `/usr/share/doc/raven/` after package installation
- **Configuration**: [Configuration Guide](docs/Configuration.md)
- **API**: REST API documentation (coming soon)
- **Examples**: Prometheus integration examples in `config/`

## Integration

### Prometheus & Grafana

Raven exposes Prometheus metrics at `/metrics`:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'raven'
    static_configs:
      - targets: ['localhost:8000']
```

Alert rules and dashboard examples included in `/usr/share/doc/raven/examples/`.

### Home Assistant

Integration with Home Assistant for smart home monitoring (coming soon).

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

See the [LICENSE](LICENSE) file for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/John-MustangGT/raven2/issues)
- **Documentation**: `/usr/share/doc/raven/` (after installation)
- **Examples**: [config/](config/) directory

---

**Raven v2.0** - Simple, fast, and reliable network monitoring for the modern connected home.
