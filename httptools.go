package main

import (
	"crypto/tls"
	"net/http"
	"net/http/cookiejar"
	"net/url"
)

type MyTransport struct {
	*http.Transport
	Agent string
}

func (mt *MyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("User-Agent", mt.Agent)
	return mt.Transport.RoundTrip(r)
}

func NewHTTPClient() *http.Client {
	transport := &MyTransport{
		Transport: http.DefaultTransport.(*http.Transport),
		Agent:     userAgent,
	}

	if debug {
		proxyURL, err := url.Parse("http://10.10.10.10:8080")
		if err != nil {
			panic(err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	cookies, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}

	res := &http.Client{
		Transport: transport,
		Jar:       cookies,
	}

	return res
}

func URLMustParse(rawurl string) *url.URL {
	url, err := url.Parse(rawurl)
	if err != nil {
		panic(err)
	}
	return url
}
