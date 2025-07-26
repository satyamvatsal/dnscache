# Ultra-Fast DNS Caching Server (Go)

This project is a high-performance DNS caching server written in Go. Its primary goal is microsecond-level response latency on cache hits. It supports both `A` and `AAAA` record types and uses an in-memory LRU cache. It forwards uncached queries to a trusted upstream DNS resolver.

---

## Features

* ⚡ **Microsecond-level cache latency**
* 📦 **LRU cache** with TTL-aware expiration
* 🔀 **Supports A and AAAA DNS queries**
* 🌐 **Upstream fallback** to `1.1.1.1` (Cloudflare)
* 🛠️ **Deployable via systemd**

---

## Upstream DNS Provider

* **Provider**: Cloudflare
* **Address**: `1.1.1.1`
* **Port**: `53`

---

## Performance Benchmark

### Tool Used

* **Tool**: `dnsperf`
* **Command**:

```bash
dnsperf -s 127.0.0.1 -d formatted_domains.txt -l 20
```

### Result:

```
Queries sent:         2,611,659
Queries completed:    2,611,659
Queries lost:         0
Response codes:       NOERROR 100%
QPS:                  130,578
Average latency:      740 µs
Latency min:          23 µs
Latency max:          530 ms
```

> This benchmark was conducted under ideal cache-hit conditions with optimized network stack settings on a local machine.

---

## How to Build

```bash
go build -o dnscache main.go
```

## How to Run

```bash
./dnscache
```

Listens on UDP port `5523`.

---

## Systemd Service Setup

1. **Install the binary**

```bash
sudo cp dnscache /usr/bin/
sudo chmod +x /usr/bin/dnscache
```

2. **Create a service file**

```ini
# /etc/systemd/system/dnscache.service
[Unit]
Description=Ultra-fast DNS caching server written in Go
After=network.target

[Service]
ExecStart=/usr/bin/dnscache
Restart=on-failure
User=nobody
Group=nogroup

[Install]
WantedBy=multi-user.target
```

3. **Enable and Start**

```bash
sudo systemctl daemon-reexec
sudo systemctl daemon-reload
sudo systemctl enable dnscache
sudo systemctl start dnscache
```

---

## Notes

* Only queries with `A` and `AAAA` types are currently cached.
* Uncached queries are resolved via Cloudflare (`1.1.1.1:53`).
* Cache TTL is derived from the lowest TTL of all `Answer` RRs.
* Default cache size is 1024 entries (LRU).
