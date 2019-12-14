package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh/terminal"
)

type AuthData struct {
	Creds   *Creds
	Cookies BasicCookies
}

type BasicCookie struct {
	Name  string
	Value string
}

type BasicCookies []*BasicCookie

type Creds struct {
	Email    string
	Password string
}

func (creds *Creds) Auth() (uploads *http.Response, err error) {
	if creds == nil || (creds.Email == "" && creds.Password == "") {
		return nil, errors.New("no credentials")
	}

	postdata := url.Values{
		"then":     {"uploads"},
		"email":    {creds.Email},
		"password": {creds.Password},
	}

	resp, err := client.PostForm("https://overcast.fm/login", postdata)
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			resp.Body.Close()
		}
	}()

	if resp.StatusCode != 200 {
		err = errors.Errorf("unexpected HTTP response on login: %d", resp.StatusCode)
		return
	}

	if strings.HasSuffix(resp.Request.URL.Path, "login") {
		err = errors.New("failed to login: wrong password")
		return
	}

	if !strings.HasSuffix(resp.Request.URL.Path, "uploads") {
		err = errors.Errorf("failed to login: request in unknown place %s", resp.Request.URL.String())
		return
	}

	return resp, nil
}

func (bc BasicCookies) Auth() (uploads *http.Response, err error) {
	if len(bc) == 0 {
		return nil, errors.New("no cookies found")
	}

	cookies := make([]*http.Cookie, len(bc))
	for i, cookie := range bc {
		cookies[i] = &http.Cookie{
			Name:  cookie.Name,
			Value: cookie.Value,
		}
	}
	client.Jar.SetCookies(overcastURL, cookies)

	uploads, err = client.Get("https://overcast.fm/uploads")
	if err != nil {
		err = errors.WithMessage(err, "failed to load uploads page")
		return
	}
	defer func() {
		if err != nil {
			uploads.Body.Close()
			uploads = nil
		}
	}()

	if !strings.HasSuffix(uploads.Request.URL.Path, "uploads") {
		err = errors.New("cookies have expired")
		return
	}

	return
}

var staleCookiesErr = errors.New("Cookies are stale, but password had worked")

func (ad *AuthData) Auth() (uploads *http.Response, err error) {
	defer func() {
		// on successful authorization
		if err == nil || err == staleCookiesErr {
			ad.saveCookies()
		}
	}()

	if ad == nil {
		return nil, errors.New("no auth data found")
	}
	if len(ad.Cookies) != 0 {
		uploads, err = ad.Cookies.Auth()
		if err == nil {
			return
		}

	}
	if ad.Creds != nil {
		uploads, err = ad.Creds.Auth()
		if err == nil {
			if len(ad.Cookies) != 0 {
				err = staleCookiesErr
			}
			return
		}
	}
	return
}

func (ad *AuthData) saveCookies() {
	cookies := client.Jar.Cookies(overcastURL)
	ad.Cookies = make([]*BasicCookie, len(cookies))
	for i, cookie := range cookies {
		ad.Cookies[i] = &BasicCookie{
			Name:  cookie.Name,
			Value: cookie.Value,
		}
	}
}

func (ad *AuthData) GetCookies() []*http.Cookie {
	res := make([]*http.Cookie, len(ad.Cookies))

	for i, cookie := range ad.Cookies {
		res[i] = &http.Cookie{
			Name:  cookie.Name,
			Value: cookie.Value,
		}
	}

	return res
}

func ParseAuthData(data []byte) (res *AuthData, err error) {
	res = &AuthData{}
	err = json.Unmarshal(data, &res)
	return
}

func inputCreds() (creds *Creds) {
	creds = &Creds{}
	var err error
	creds.Email, err = Input("Email: ")
	if err != nil {
		fmt.Printf("[WARN] Failed to read email: %s\n", err)
		return nil
	}

	fmt.Print("Password: ")
	var bytePassword []byte
	bytePassword, err = terminal.ReadPassword(int(syscall.Stdin))
	fmt.Print("\n")

	if err != nil {
		fmt.Printf("[WARN] Failed to read password: %s\n", err)
		return nil
	} else {
		creds.Password = strings.TrimSpace(string(bytePassword))
	}
	return
}
