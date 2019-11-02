package main

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh/terminal"
)

type Creds struct {
	Email    string
	Password string
}

func login(creds *Creds) (err error) {
	postdata := url.Values{
		"then":     {"uploads"},
		"email":    {creds.Email},
		"password": {creds.Password},
	}

	resp, err := client.PostForm("https://overcast.fm/login", postdata)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.Errorf("Unexpected HTTP response on login: %d", resp.StatusCode)
	}

	if strings.HasSuffix(resp.Request.URL.Path, "login") {
		return errors.New("Failed to login: wrong password")
	}

	if !strings.HasSuffix(resp.Request.URL.Path, "uploads") {
		return errors.Errorf("Failed to login: Request in unknown place %s", resp.Request.URL.String())
	}

	return parseUploadsPage(resp.Body)
}

func requestPassword() (err error) {
	creds := &Creds{}
	creds.Email, creds.Password, err = inputCreds()
	if err != nil {
		return
	}

	err = login(creds)
	if err != nil {
		return
	}

	if args.StorePassword {
		config.Creds = creds
	}

	return
}

func auth() (err error) {
	if args.StorePassword && !args.Silent {
		return requestPassword()
	}

	if len(config.Cookies) != 0 {
		// attempting to access with saved cookies
		var resp *http.Response
		resp, err = client.Get("https://overcast.fm/uploads")
		if err == nil {
			defer resp.Body.Close()

			if strings.HasSuffix(resp.Request.URL.Path, "uploads") {
				// Wasn't redirected to /login, so cookies are valid
				return parseUploadsPage(resp.Body)
			}
		}
		printf("[WARN] Failed to log in with stored cookies\n")
	}

	if config.Creds != nil {
		// Got stored credentials, using them to login
		err = login(config.Creds)
		if err != nil {
			printf("[WARN] Failed to log in with stored credentials (%s)\n", err)
		}
	}
	if !args.Silent {
		return requestPassword()
	}
	return errors.New("Failed to login")
}

func inputCreds() (username, password string, err error) {
	reader := bufio.NewReader(os.Stdin)
	if args.Login != "" {
		fmt.Print("Email: ")
		username, err = reader.ReadString('\n')
		if err != nil {
			return
		}
	} else {
		username = args.Login
	}
	if args.Password != "" {
		fmt.Print("Password: ")
		var bytePassword []byte
		bytePassword, err = terminal.ReadPassword(int(syscall.Stdin))
		if err != nil {
			err = errors.Wrap(err, "Failed to enter the password")
			return
		}
		fmt.Print("\n")
		password = string(bytePassword)
	} else {
		password = args.Password
	}

	return
}
