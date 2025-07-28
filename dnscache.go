package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
	"gopkg.in/yaml.v3"
)

type CacheEntry struct {
	Msg     *dns.Msg
	Expires time.Time
}

type Config struct {
	ListenPort    int               `yaml:"listen_port"`
	CacheSize     int               `yaml:"cache_size"`
	DefaultTtl    uint32            `yaml:"default_ttl"`
	UpstreamDNS   []string          `yaml:"upstream_dns"`
	CustomRecords map[string]string `yaml:"custom_records"`
}

var (
	cache         *lru.Cache
	customRecords map[string]net.IP
	upstreams     []string
	defaultTTL    uint32
)

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	return &cfg, err
}

func main() {

	cfg, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatal("Error loading config:", err)
	}

	defaultTTL = cfg.DefaultTtl
	upstreams = cfg.UpstreamDNS
	customRecords = make(map[string]net.IP)

	for domain, ip := range cfg.CustomRecords {
		parsed := net.ParseIP(ip)
		if parsed == nil {
			log.Fatalf("Invalid IP %s for domain %s", ip, domain)
		}
		customRecords[domain] = parsed
	}

	cache, err = lru.New(cfg.CacheSize)
	if err != nil {
		log.Fatal(err)
	}

	dns.HandleFunc(".", handleRequest)

	addr := fmt.Sprintf(":%d", cfg.ListenPort)
	server := &dns.Server{
		Addr: addr,
		Net:  "udp",
	}

	log.Println("Starting DNS server at port", addr)
	err = server.ListenAndServe()
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

}

func handleRequest(w dns.ResponseWriter, r *dns.Msg) {

	if len(r.Question) == 0 {
		return
	}

	q := r.Question[0]
	cacheKey := q.Name + ":" + dns.TypeToString[q.Qtype]

	// custom record check
	if ip, ok := customRecords[q.Name]; ok && q.Qtype == dns.TypeA {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Authoritative = true
		a := &dns.A{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    defaultTTL,
			},
			A: ip,
		}
		m.Answer = append(m.Answer, a)
		w.WriteMsg(m)
		return
	}

	//check cache
	if val, ok := cache.Get(cacheKey); ok {
		entry := val.(CacheEntry)
		if time.Now().Before(entry.Expires) {
			resp := entry.Msg.Copy()
			resp.SetReply(r)
			resp.Id = r.Id
			w.WriteMsg(resp)
			return
		}
		cache.Remove(cacheKey)
	}

	var resp *dns.Msg

	var err error

	for _, upstream := range upstreams {
		c := new(dns.Client)
		resp, _, err = c.Exchange(r, upstream)
		if err != nil && resp != nil {
			break
		}
	}

	if err != nil || resp == nil {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(m)
		return
	}

	// determine ttl

	ttl := defaultTTL
	for _, rr := range resp.Answer {
		if rr.Header().Ttl < ttl {
			ttl = rr.Header().Ttl
		}
	}

	cache.Add(cacheKey, CacheEntry{
		Msg:     resp.Copy(),
		Expires: time.Now().Add(time.Duration(ttl) * time.Second),
	})

	w.WriteMsg(resp)
}
