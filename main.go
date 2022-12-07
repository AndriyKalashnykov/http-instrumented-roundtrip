package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"time"
)

var Show bool
var tp *customTransport
var client *http.Client

type customTransport struct {
	rtp       http.RoundTripper
	dialer    *net.Dialer
	connStart time.Time
	connEnd   time.Time
	reqStart  time.Time
	reqEnd    time.Time
}

func newTransport() *customTransport {

	tr := &customTransport{
		dialer: &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		},
	}
	tr.rtp = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		//Dial:                tr.dial,
		DialContext:         tr.dialContext,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		DisableKeepAlives:   true,
	}
	return tr
}

func (tr *customTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	tr.reqStart = time.Now()
	resp, err := tr.rtp.RoundTrip(r)
	tr.reqEnd = time.Now()
	return resp, err
}

func (tr *customTransport) dial(network, addr string) (net.Conn, error) {
	tr.connStart = time.Now()
	cn, err := tr.dialer.Dial(network, addr)
	tr.connEnd = time.Now()
	return cn, err
}
func (tr *customTransport) dialContext(context context.Context, network, addr string) (net.Conn, error) {
	tr.connStart = time.Now()
	cn, err := tr.dialer.DialContext(context, network, addr)
	tr.connEnd = time.Now()
	return cn, err
}

func (tr *customTransport) ReqDuration() time.Duration {
	return tr.Duration() - tr.ConnDuration()
}

func (tr *customTransport) ConnDuration() time.Duration {
	return tr.connEnd.Sub(tr.connStart)
}

func (tr *customTransport) Duration() time.Duration {
	return tr.reqEnd.Sub(tr.reqStart)
}

func MakeHttpCall(client *http.Client, req *http.Request, url string) {
	//resp, err := client.Get(url)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("get error: %s: %s", err, url)
	}
	defer resp.Body.Close()

	output := ioutil.Discard
	if Show {
		output = os.Stdout
	}
	io.Copy(output, resp.Body)

	if Show {
		log.Println("Duration:", tp.Duration())
		log.Println("Request duration:", tp.ReqDuration())
		log.Println("Connection duration:", tp.ConnDuration())
	}
}

func main() {

	flag.BoolVar(&Show, "show", true, "Display the response content")
	flag.Parse()

	//url := flag.Args()[0]
	//url := "http://localhost:1338/hello"
	url := "https://httpbin.org/get"

	fmt.Println("URL:", url)

	tp = newTransport()
	client = &http.Client{Transport: tp}

	clientTrace := &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			log.Printf("Connection was reused: %t", info.Reused)
		},
		DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
			log.Printf("DNS Info: %+v\n", dnsInfo)
		},
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}

	instrumentedReq := req.WithContext(httptrace.WithClientTrace(req.Context(), clientTrace))

	MakeHttpCall(client, instrumentedReq, url)
	MakeHttpCall(client, instrumentedReq, url)
	MakeHttpCall(client, instrumentedReq, url)
}
