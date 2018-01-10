package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/miekg/dns"
	cache "github.com/patrickmn/go-cache"
	"golang.org/x/net/publicsuffix"
)

var (
	version string
	build   string
	date    string
)

var path string
var logflag bool
var mapv4 map[string]string
var mapv6 map[string]string
var maptldv4 map[string]string
var maptldv6 map[string]string
var client dns.Client
var resolvConf dns.ClientConfig
var c *cache.Cache

func main() {
	if version == "" {
		version = "1.0.0"
	}
	if build == "" {
		build = "debug"
	}
	if date == "" {
		date = time.Now().UTC().String()
	}

	log.Println(" --------------------------------------------------- ")
	log.Printf(" Version: %s", version)
	log.Printf(" Build: %s", build)
	log.Printf(" Date: %s", date)
	log.Println(" --------------------------------------------------- ")

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	flag.BoolVar(&logflag, "log", false, "Log requests to stdout")
	flag.StringVar(&path, "path", dir+"/configuration.json", "Config File")
	flag.Parse()

	conf := GetConfiguration(path)

	mapv4 = make(map[string]string)
	mapv6 = make(map[string]string)
	for _, val := range conf.Hosts {
		mapv4[val.Key] = val.Ipv4
		mapv6[val.Key] = val.Ipv6
	}
	maptldv4 = make(map[string]string)
	maptldv6 = make(map[string]string)
	for _, val := range conf.TLDS {
		maptldv4[val.Key] = val.Ipv4
		maptldv6[val.Key] = val.Ipv6
	}
	resolver, _ := dns.ClientConfigFromFile(conf.Resolver)
	client := new(dns.Client)
	c = cache.New(5*time.Minute, 10*time.Minute)
	dns.HandleFunc(".", middleware(conf, client, resolver))
	go func() {
		srv := &dns.Server{Addr: conf.Bind.UDP, Net: "udp"}
		err := srv.ListenAndServe()
		if err != nil {
			log.Fatalf("Failed to set udp listener %s\n", err.Error())
		}
	}()
	go func() {
		srv := &dns.Server{Addr: conf.Bind.TCP, Net: "tcp"}
		err := srv.ListenAndServe()
		if err != nil {
			log.Fatalf("Failed to set tcp listener %s\n", err.Error())
		}
	}()

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case s := <-sig:
			log.Fatalf("Signal (%d) received, stopping\n", s)
		}
	}
}

func middleware(conf *Configuration, client *dns.Client, resolvConf *dns.ClientConfig) func(dns.ResponseWriter, *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		domain := r.Question[0].Name
		domainWithoutDot := strings.TrimSuffix(domain, ".")
		tld, _ := publicsuffix.EffectiveTLDPlusOne(domainWithoutDot)

		t := time.Now().Format("2006-01-02T15:04:05-0700")
		ip, _, _ := net.SplitHostPort(w.RemoteAddr().String())
		protocol := w.RemoteAddr().Network()
		fmt.Printf("%s\t%s\t%s\t%s\n", t, protocol, ip, domain)

		m := new(dns.Msg)
		if val, found := c.Get(domain); found {
			msg, _ := val.(*dns.Msg)
			msg.SetReply(r)
			w.WriteMsg(msg)
		} else {
			if val, ok := mapv4[domainWithoutDot]; ok {
				rr1 := new(dns.A)
				rr1.Hdr = dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: uint32(conf.TTL)}
				rr2 := new(dns.AAAA)
				rr2.Hdr = dns.RR_Header{Name: domain, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: uint32(conf.TTL)}

				rr1.A = net.ParseIP(val)
				rr2.AAAA = net.ParseIP(mapv6[domainWithoutDot])

				m.Answer = []dns.RR{rr1, rr2}
			} else if val, ok := maptldv4[tld]; ok {
				rr1 := new(dns.A)
				rr1.Hdr = dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: uint32(conf.TTL)}
				rr2 := new(dns.AAAA)
				rr2.Hdr = dns.RR_Header{Name: domain, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: uint32(conf.TTL)}

				rr1.A = net.ParseIP(val)
				rr2.AAAA = net.ParseIP(mapv6[domainWithoutDot])

				m.Answer = []dns.RR{rr1, rr2}
			} else {
				res, _, err := client.Exchange(r, net.JoinHostPort(resolvConf.Servers[0], resolvConf.Port))
				if m == nil {
					log.Fatalf("*** error: %s\n", err.Error())
				}
				if m.Rcode != dns.RcodeSuccess {
					log.Fatalf(" *** invalid answer name %s for %s\n", os.Args[1], os.Args[1])
				}
				m.Answer = res.Answer
			}
			m.Authoritative = true
			m.SetReply(r)

			c.Add(domain, m, cache.DefaultExpiration)
			w.WriteMsg(m)
		}
	}
}
