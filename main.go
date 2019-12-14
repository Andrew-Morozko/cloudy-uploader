package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/pkg/errors"
	"github.com/shibukawa/configdir"
	"github.com/vbauerster/mpb/v4/decor"
	"github.com/zalando/go-keyring"
)

var (
	appName   = "cloudyuploader"
	version   = "0.0.0-in-dev" // overriden on build
	appURL    = "https://github.com/Andrew-Morozko/cloudy-uploader"
	userAgent = appName + "/" + version + " CLI Uploader; " + appURL
)

var overcastURL = URLMustParse("https://overcast.fm/")
var debug = false
var client = NewHTTPClient()

var allowedExts = ExtList{"mp3", "m4a", "aac", "wav", "m4b"}

type ExtList []string

func (el ExtList) Inclues(ext string) bool {
	ext = strings.ToLower(ext)
	ext = strings.TrimLeft(ext, ".")
	for _, allowed := range el {
		if allowed == ext {
			return true
		}
	}
	return false
}

type Args struct {
	Files       []string `arg:"--file,positional,required" help:"files to be uploaded"`
	MaxParallel int      `arg:"-j,--parallel-uploads" help:"maximum number of concurrent upload jobs" default:"4" placeholder:"N"`
	Login       string   `help:"email for Overcast account"`
	Password    string   `help:"password for Overcast account"`
	SaveCreds   *bool    `arg:"--save-creds" help:"save credentials in secure system storge"`
	NoLoadCreds bool     `arg:"--no-load-creds" help:"do not use stored creds"`
	Silent      bool     `arg:"-s" help:"disable user interaction"`
}

func (Args) Description() string {
	return `Unofficial CLI file uploader for Overcast. Version ` + version + `
Technically it's just a wrapper around upload a form at https://overcast.fm/uploads
`
}

func migrateToKeyring() {
	configDirs := configdir.New("", appName)
	configDir := configDirs.QueryFolders(configdir.Global)[0]

	if !configDir.Exists("config.json") {
		return
	}
	data, err := configDir.ReadFile("config.json")
	if err != nil {
		fmt.Printf("[WARN] Failed to read config file: %s\n", err)
		return
	}

	var cfg AuthData
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		fmt.Printf("[WARN] Invalid JSON config file: %s\n", err)
		return
	}

	err = keyring.Set(appName, "creds", string(data))
	if err != nil {
		fmt.Printf("[WARN] Failed to save credentials: %s\n", err)
	}
	os.RemoveAll(configDir.Path)
}

func loadCreds() (authData *AuthData) {
	migrateToKeyring()

	data, err := keyring.Get(appName, "creds")
	if err != nil {
		if err != keyring.ErrNotFound {
			fmt.Printf("[WARN] Failed to load credentials: %s\n", err)
		}
		return nil
	}
	authData, err = ParseAuthData([]byte(data))
	if err != nil {
		fmt.Printf("[WARN] Failed to load credentials: %s\n", err)
		return nil
	}
	return
}

func saveCreds(ad *AuthData) {
	data, err := json.Marshal(ad)
	if err != nil {
		return
	}

	err = keyring.Set(appName, "creds", string(data))
	if err != nil {
		fmt.Printf("[WARN] Failed to save credentials: %s\n", err)
		return
	}
}

func parseArgs() (args *Args, err error) {
	args = &Args{}

	arg.MustParse(args)

	if args.MaxParallel < 1 {
		err = errors.New("--parallel-uploads should be at least 1")
		return
	}

	if args.Silent {
		os.Stdout, err = os.Open(os.DevNull)
		if err != nil {
			err = errors.WithMessage(err, "can't open devnull")
			return
		}
	}
	return
}

func Input(prompt string) (answer string, err error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	answer, err = reader.ReadString('\n')
	if err != nil {
		return
	}
	answer = strings.TrimSpace(answer)
	return
}

func PerformAuth(args *Args) (uploads *http.Response, err error) {
	var ad *AuthData
	// auth with command line data
	if args.Login != "" && args.Password != "" {
		ad = &AuthData{
			Creds: &Creds{
				args.Login,
				args.Password,
			},
		}
		uploads, err = ad.Auth()
		if err == nil {
			if args.SaveCreds != nil && *args.SaveCreds {
				saveCreds(ad)
			}
			return
		} else {
			fmt.Printf("[WARN] Failed to authenticate with supplied login/passowrd: %s\n", err)
		}
	}

	// auth with cookies/saved password
	if !args.NoLoadCreds {
		ad = loadCreds()
		if ad != nil {
			uploads, err = ad.Auth()
			if err == staleCookiesErr {
				// refresh saved cookies
				saveCreds(ad)
				err = nil
			}
			if err == nil {
				return
			} else {
				fmt.Printf("[WARN] Failed to authenticate with saved login/passowrd: %s\n", err)
			}
		}
	}

	// auth with input password
	if !args.Silent {
		ad = &AuthData{
			Creds: inputCreds(),
		}
		if ad.Creds != nil {
			uploads, err = ad.Auth()
			if err == nil {
				if args.SaveCreds == nil {
					// Ask to save
					answer, err2 := Input("Do you want to store the email/password securely on your system? [y/N]: ")
					if err2 != nil {
						fmt.Printf("\n[WARN] Failed to get answer: %s\n", err2)
						return
					}

					if len(answer) >= 1 && (answer[0] == 'Y' || answer[0] == 'y') {
						saveCreds(ad)
					}
				} else {
					if *args.SaveCreds {
						saveCreds(ad)
					}
				}
				return
			} else {
				fmt.Printf("[WARN] Failed to authenticate with entered login/passowrd: %s\n", err)
			}
		}
	}
	uploads = nil
	err = errors.New("all availible methods failed")
	return
}
func parseFiles(files []string, overcastParams *OvercastParams) (jobs []*Job) {
	for _, file := range files {
		if !allowedExts.Inclues(filepath.Ext(file)) {
			fmt.Printf("[WARN] File \"%s\" is not allowed. Allowed extentions: %s\n", file, strings.Join(allowedExts, ", "))
			continue
		}

		stat, err := os.Stat(file)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Printf("[WARN] File \"%s\" doesn't exist\n", file)
			} else {
				fmt.Printf("[WARN] Error with file \"%s\": %s\n", file, err)
			}
			continue
		}

		fileSize := stat.Size()
		if fileSize > overcastParams.MaxFileSize {
			fmt.Printf(
				"[WARN] File \"%s\" is too large: file size=% .2f, max file size=% .2f\n",
				file, decor.SizeB1000(fileSize), decor.SizeB1000(overcastParams.MaxFileSize),
			)
			continue
		}
		jobs = append(jobs, NewJob(file, fileSize))
	}
	return
}

func main() {
	var err error

	defer func() {
		if err != nil {
			fmt.Printf("[ERROR] %s\n", err)
			os.Exit(-1)
		}
	}()

	args, err := parseArgs()
	if err != nil {
		err = errors.WithMessage(err, "Arguments error")
		return
	}

	upl, err := PerformAuth(args)
	if err != nil {
		err = errors.WithMessage(err, "Auth failed")
		return
	}

	overcastParams, err := parseUploadsPage(upl.Body)
	upl.Body.Close()
	if err != nil {
		err = errors.WithMessage(err, "Failed to parse the uploads page")
		return
	}

	jobs := parseFiles(args.Files, overcastParams)

	if len(jobs) == 0 {
		err = errors.New("No files to upload!")
		return
	}

	if len(jobs) > overcastParams.MaxFileCount {
		err = errors.Errorf("You've chosen too many files(%d), you only have %d files remaining",
			len(jobs), overcastParams.MaxFileCount,
		)
		return
	}

	var totalSize int64
	for _, job := range jobs {
		totalSize += job.FileSize
	}

	if totalSize > overcastParams.SpaceAvailible {
		err = errors.Errorf("Files are too large: total size=% .2f; you have % .2f availible\n",
			decor.SizeB1000(totalSize), decor.SizeB1000(overcastParams.SpaceAvailible),
		)
		return
	}

	performUpload(jobs, args.MaxParallel, overcastParams)
}
