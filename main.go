package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type event rune

const (
	START event = '*'
	ERROR event = '!'
	OPEN  event = '+'
	CLOSE event = '-'
)

const app = "supgrade"

var (
	port *int
	dest *string
	verb *bool
)

func init() {
	port = flag.Int("p", 80, "port to listen on")
	dest = flag.String("d", "", "forwarding destination")
	verb = flag.Bool("v", false, "verbose errors")

	flag.Parse()
}

func main() {
	var (
		dhost string
		dport int
	)

	if *dest == "" {
		die("expected destination")
	} else if u, err := url.Parse("scheme://" + *dest); err != nil {
		die("destination", err.Error())
	} else {
		dhost = u.Hostname()

		if p := u.Port(); p == "" {
			dport = 443
		} else if n, err := strconv.ParseInt(p, 10, 16); err != nil {
			die("destination port", err.Error())
		} else {
			dport = int(n)
		}

		*dest = fmt.Sprintf("%s:%d", dhost, dport)
	}

	listen := fmt.Sprintf("0.0.0.0:%d", *port)

	if socket, err := net.Listen("tcp", listen); err != nil {
		die(err.Error())
	} else {
		log(START, listen, *dest)
		for {
			if conn, err := socket.Accept(); err != nil {
				log(ERROR, listen, *dest, err)
			} else {
				go handle(conn, dhost, dport)
			}
		}
	}
}

func handle(src net.Conn, host string, port int) {
	forward := fmt.Sprintf("%s:%d", host, port)

	if dst, err := net.Dial("tcp", forward); err != nil {
		log(ERROR, src.RemoteAddr().String(), forward, err)
		src.Close()
	} else {
		s, d := src.RemoteAddr().String(), dst.RemoteAddr().String()
		log(OPEN, s, d)
		dst = tls.Client(dst, &tls.Config{ServerName: host})
		w := pipe(src, dst)
		w.Wait()
		log(CLOSE, s, d)
	}
}

func pipe(src, dst net.Conn) *sync.WaitGroup {
	w := new(sync.WaitGroup)
	w.Add(2)

	go func() {
		defer w.Done()
		io.Copy(src, dst)
		src.Close()
		dst.Close()
	}()

	go func() {
		defer w.Done()
		io.Copy(dst, src)
		src.Close()
		dst.Close()
	}()

	return w
}

func log(kind event, src, dst string, err ...error) {
	fmt.Fprintf(
		os.Stdout,
		"%s [%s] %s => %s\n",
		time.Now().In(time.UTC).Format(time.RFC3339),
		string(kind),
		src, dst,
	)

	if kind == ERROR && *verb && len(err) > 0 {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "%s\n", err)
		fmt.Fprintln(os.Stderr)
	}
}

func die(messages ...string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", app, strings.Join(messages, ": "))
	os.Exit(1)
}
