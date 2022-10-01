package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"github.com/miekg/dns"
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
	name *string
	nett *time.Duration
	zone *string
)

var parsedZone *time.Location

func init() {
	port = flag.Int("p", 80, "port to listen on")
	dest = flag.String("d", "", "forwarding destination")
	verb = flag.Bool("v", false, "verbose errors")
	name = flag.String("n", "", "nameserver to use for looking up destination")
	nett = flag.Duration("t", 10*time.Second, "timeout for network operations")
	zone = flag.String("z", "", "time zone for which to display timestamps")
}

func main() {
	flag.Parse()

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

	if *zone == "" {
		parsedZone = time.Local
	} else if z, err := time.LoadLocation(*zone); err != nil {
		die("invalid time zone", err.Error())
	} else {
		parsedZone = z
	}

	listen := fmt.Sprintf("0.0.0.0:%d", *port)

	if socket, err := net.Listen("tcp", listen); err != nil {
		die(err.Error())
	} else {
		log(START, listen, *dest, nil)
		for {
			if conn, err := socket.Accept(); err != nil {
				log(ERROR, listen, *dest, err)
			} else {
				go handle(conn, *name, dhost, dport)
			}
		}
	}
}

func handle(src net.Conn, ns, host string, port int) {
	var forward string

	if ns != "" {
		if ip, err := lookup(ns, host); err != nil {
			log(ERROR, src.RemoteAddr().String(), host, err)
			return
		} else {
			forward = fmt.Sprintf("%s:%d", ip, port)
		}
	} else {
		forward = fmt.Sprintf("%s:%d", host, port)
	}

	if dst, err := net.DialTimeout("tcp", forward, *nett); err != nil {
		log(ERROR, src.RemoteAddr().String(), forward, err)
		src.Close()
	} else {
		s, d := src.RemoteAddr().String(), dst.RemoteAddr().String()
		log(OPEN, s, d, nil)
		dst = tls.Client(dst, &tls.Config{ServerName: host})
		w := pipe(src, dst)
		w.Wait()
		log(CLOSE, s, d, nil)
	}
}

func lookup(ns, host string) (string, error) {
	c := dns.Client{
		DialTimeout:  *nett,
		Timeout:      *nett,
		ReadTimeout:  *nett,
		WriteTimeout: *nett,
	}

	m := dns.Msg{}
	m.SetQuestion(host+".", dns.TypeA)

	r, _, err := c.Exchange(&m, ns+":53")

	if err != nil {
		return "", err
	}

	if len(r.Answer) == 0 {
		return "", errors.New("no results from resolver")
	}

	for _, a := range r.Answer {
		if a, ok := a.(*dns.A); ok {
			return a.A.String(), nil
		}
	}

	return "", errors.New("no A records found for destination")
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

func log(kind event, src, dst string, err error) {
	fmt.Fprintf(
		os.Stdout,
		"%s [%s] %s => %s\n",
		time.Now().In(parsedZone).Format(time.RFC3339),
		string(kind),
		src, dst,
	)

	if kind == ERROR && *verb && err != nil {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "%s\n", err)
		fmt.Fprintln(os.Stderr)
	}
}

func die(messages ...string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", app, strings.Join(messages, ": "))
	os.Exit(1)
}
