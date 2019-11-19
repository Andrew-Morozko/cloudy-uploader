package main

import (
	"crypto/tls"
	"net/http"
	"net/http/cookiejar"
	"net/url"
)

type credState int

const (
	credStateUnchanged credState = iota
	credStateModified
	credStateReplaced
)

type AuthData struct {
	Creds   *Creds
	Cookies []*BasicCookie
	state   credState
}

type BasicCookie struct {
	Name  string
	Value string
}

func (c *AuthData) SetCookies(cookies []*http.Cookie) {
	c.Cookies = make([]*BasicCookie, len(cookies))
	for i, cookie := range cookies {
		c.Cookies[i] = &BasicCookie{
			Name:  cookie.Name,
			Value: cookie.Value,
		}
	}
}

func (c *AuthData) GetCookies() []*http.Cookie {
	res := make([]*http.Cookie, len(c.Cookies))

	for i, cookie := range c.Cookies {
		res[i] = &http.Cookie{
			Name:  cookie.Name,
			Value: cookie.Value,
		}
	}

	return res
}

type MyTransport struct {
	*http.Transport
	Agent string
}

func (mt *MyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("User-Agent", mt.Agent)
	return mt.Transport.RoundTrip(r)
}

var client = NewHTTPClient()

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
