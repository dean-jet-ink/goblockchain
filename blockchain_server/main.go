package main

import (
	"flag"
	"log"
)

func init() {
	log.SetPrefix("BlockChain: ")
}

func main() {
	port := flag.Uint("port", 5000, "TCP port number for Blockchain Server")
	flag.Parse()
	bcs := NewBlockchainServer(uint16(*port))
	bcs.Start()
}
