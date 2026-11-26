// agent runs on the public side (Pterodactyl) and owns the single reachable port.
// That port carries two kinds of connections: the tunnel client dialing in, and
// people SSHing in. The first line tells them apart.
package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/xtaci/smux"
)

const magic = "RT1"

// conn lets smux/user relaying read from a bufio.Reader that already consumed
// the handshake line.
type conn struct {
	net.Conn
	r io.Reader
}

func (c *conn) Read(p []byte) (int, error) { return c.r.Read(p) }

type tunnel struct {
	mu  sync.Mutex
	ses *smux.Session
}

func (t *tunnel) set(s *smux.Session) {
	t.mu.Lock()
	old := t.ses
	t.ses = s
	t.mu.Unlock()
	if old != nil {
		old.Close()
	}
}

func (t *tunnel) get() *smux.Session {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.ses
}

func main() {
	listen := flag.String("listen", ":"+envOr("SERVER_PORT", "14776"), "listen address")
	token := flag.String("token", os.Getenv("TUNNEL_TOKEN"), "shared client token")
	flag.Parse()
	if *token == "" {
		log.Fatal("set TUNNEL_TOKEN (or -token); without it anyone can claim the tunnel")
	}
	addr := *listen
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("agent listening on %s", ln.Addr())

	t := &tunnel{}
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go handle(c, *token, t)
	}
}

func handle(c net.Conn, token string, t *tunnel) {
	br := bufio.NewReader(c)
	c.SetReadDeadline(time.Now().Add(10 * time.Second))
	line, err := br.ReadString('\n')
	c.SetReadDeadline(time.Time{})
	if err != nil {
		c.Close()
		return
	}

	if line == magic+" "+token+"\n" {
		ses, err := smux.Server(&conn{Conn: c, r: br}, nil)
		if err != nil {
			log.Print(err)
			c.Close()
			return
		}
		log.Printf("tunnel client connected from %s", c.RemoteAddr())
		t.set(ses)
		<-ses.CloseChan()
		log.Printf("tunnel client from %s gone", c.RemoteAddr())
		return
	}

	// anything else is a user connection; put the consumed line back in front
	ses := t.get()
	if ses == nil {
		log.Printf("no tunnel client, dropping %s", c.RemoteAddr())
		c.Close()
		return
	}
	stream, err := ses.OpenStream()
	if err != nil {
		log.Print(err)
		c.Close()
		return
	}
	user := &conn{Conn: c, r: io.MultiReader(strings.NewReader(line), br)}
	relay(user, stream)
}

func relay(a, b io.ReadWriteCloser) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { io.Copy(a, b); a.Close(); wg.Done() }()
	go func() { io.Copy(b, a); b.Close(); wg.Done() }()
	wg.Wait()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
