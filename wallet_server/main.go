package main

import "flag"

func main() {
	port := flag.Uint("port", 8080, "TCP port number for Wallet Server")
	gateway := flag.String("gateway", "http://localhost:5000", "Blockchain Gateway")
	flag.Parse()

	ws := NewWalletServer(uint16(*port), *gateway)
	ws.Start()
}
