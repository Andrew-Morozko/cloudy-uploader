package main

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/url"
)


type AuthData struct {
	Creds   *Creds
	Cookies []*BasicCookie
	hash    [32]byte
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

func (c *AuthData) Marshal() ([]byte, error) {
	return json.Marshal(authData)
}

func (c *AuthData) Changed() bool {
	data, err := json.Marshal(authData)
	if err != nil {
		return false
	}
	return c.hash != sha256.Sum256(data)
}

func NewAuthData(data []byte) (res *AuthData, err error) {
	res = &AuthData{}
	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	// To have data in a normalized format
	data, err = json.Marshal(authData)
	if err != nil {
		return nil, err
	}
	res.hash = sha256.Sum256(data)
	return
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
