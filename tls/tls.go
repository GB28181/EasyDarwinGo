package tls

import (
	"crypto/tls"
	"log"
	"net"
)

func NewTlsListener(certFile string, keyFile string, addr *net.TCPAddr) (net.Listener, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	config := &tls.Config{Certificates: []tls.Certificate{cert}}

	listener, err := net.ListenTCP("tcp", addr)
	ln := tls.NewListener(listener, config)

	return ln, nil
}
