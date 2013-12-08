package main

import (
	// "fmt"
	"log"
	"net"
	"os"

	"printer"
	"d8/client"
	. "d8/domain"
	. "d8/packet/consts"
)

func noError(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func main() {
	q := &client.Query{
		Domain: D("liulonnie.net"),
		Type:   A,
		Server: &net.UDPAddr{IP: net.ParseIP("74.220.195.131")},
		Printer: printer.New(os.Stdout),
	}

	client, e := client.New()
	noError(e)

	client.Query(q)
}
