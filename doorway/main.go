package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"golang.org/x/crypto/acme/autocert"
)

const contactEmail = "lonnie+rayci@anyscale.com"

func run(domain, proxyTo, certDir string) error {
	// serve https on domain, use letsencrypt for certs
	// and forward all connections to proxyTo

	if err := os.MkdirAll(certDir, 0755); err != nil {
		return fmt.Errorf("make cert cache dir: %w", err)
	}

	m := &autocert.Manager{
		Cache:      autocert.DirCache(certDir),
		Prompt:     autocert.AcceptTOS,
		Email:      contactEmail,
		HostPolicy: autocert.HostWhitelist(domain),
	}

	proxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   proxyTo,
	})

	const serveAddr = ":8443"

	s := &http.Server{
		Addr:      serveAddr,
		TLSConfig: m.TLSConfig(),
		Handler:   proxy,
	}

	log.Printf("serving at %s", serveAddr)
	log.Printf("forwarding to %s", proxyTo)

	return s.ListenAndServeTLS("", "")
}

func main() {
	domain := flag.String("domain", "ci.ray.io", "domain to serve")
	proxyTo := flag.String("proxy_to", "reefd:8000", "forward to this address")
	certDir := flag.String("cert_dir", "var/certs", "cache dir for certs")
	flag.Parse()

	if err := run(*domain, *proxyTo, *certDir); err != nil {
		log.Fatal(err)
	}
}
