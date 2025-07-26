package main

import (
	"log"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
)

type CacheEntry struct {
	Msg     *dns.Msg
	Expires time.Time
}

var (
	cache *lru.Cache
)

func main() {
	var err error
	cache, err = lru.New(1024)

	if err != nil {
		log.Fatal(err)
	}

	dns.HandleFunc(".", handleRequest)

	server := &dns.Server{
		Addr: ":5354",
		Net:  "udp",
	}

	log.Println("Starting DNS server at port", server.Addr)
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

	// cache check
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

	c := new(dns.Client)
	resp, _, err := c.Exchange(r, "1.1.1.1:53")
	if err != nil || resp == nil {
		m := new(dns.Msg)
		m.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(m)
		return
	}

	ttl := uint32(60)
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
