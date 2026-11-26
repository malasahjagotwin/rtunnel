// client runs on the container you want to reach. It dials out to the agent so
// no inbound port is needed here, then exposes a local address (sshd) through
// the agent's public port.
package main

import (
	"flag"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/xtaci/smux"
)

func main() {
	server := flag.String("server", "", "agent address host:port (required)")
	token := flag.String("token", os.Getenv("TUNNEL_TOKEN"), "shared client token")
	target := flag.String("target", "127.0.0.1:22", "local address to expose")
	flag.Parse()
	if *server == "" || *token == "" {
		log.Fatal("need -server and -token (or TUNNEL_TOKEN)")
	}

	backoff := time.Second
	for {
		start := time.Now()
		err := run(*server, *token, *target)
		if time.Since(start) > time.Minute {
			backoff = time.Second
		}
		log.Printf("tunnel down: %v (retry in %s)", err, backoff)
		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func run(server, token, target string) error {
	c, err := net.DialTimeout("tcp", server, 10*time.Second)
	if err != nil {
		return err
	}
	if _, err := c.Write([]byte("RT1 " + token + "\n")); err != nil {
		c.Close()
		return err
	}
	ses, err := smux.Client(c, nil)
	if err != nil {
		c.Close()
		return err
	}
	defer c.Close()
	log.Printf("tunnel up to %s, exposing %s", server, target)

	for {
		stream, err := ses.AcceptStream()
		if err != nil {
			return err
		}
		go func() {
			local, err := net.DialTimeout("tcp", target, 10*time.Second)
			if err != nil {
				log.Printf("dial %s: %v", target, err)
				stream.Close()
				return
			}
			relay(local, stream)
		}()
	}
}

func relay(a, b io.ReadWriteCloser) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { io.Copy(a, b); a.Close(); wg.Done() }()
	go func() { io.Copy(b, a); b.Close(); wg.Done() }()
	wg.Wait()
}
