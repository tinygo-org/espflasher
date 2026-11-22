//go:build !(js && wasm)

// Run with "go run ." to serve this directory for the browser.
package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "listen address")
	flag.Parse()
	log.Printf("serving on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, http.FileServer(http.Dir("."))))
}
