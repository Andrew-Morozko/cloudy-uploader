package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/shibukawa/configdir"
	"github.com/zalando/go-keyring"
)

var (
	appName   = "cloudyuploader"
	version   = "0.0.0-in-dev" // overriden on build
	appURL    = "https://github.com/Andrew-Morozko/cloudy-uploader"
	userAgent = appName + "/" + version + " CLI Uploader; " + appURL
)

var overcastURL *url.URL

var authData AuthData
var debug = false

type Args struct {
	Files       []string `arg:"--file,positional,required" help:"files to be uploaded"`
	MaxParallel int      `arg:"-j,--parallel-uploads" help:"maximum number of concurrent upload jobs" default:"4" placeholder:"N"`
	Login       string   `help:"email for Overcast account"`
	Password    string   `help:"password for Overcast account"`
	SaveCreds   bool     `arg:"--save-creds" help:"save credentials in secure system storge" default:"true"`
	Silent      bool     `arg:"-s" help:"disable user interaction"`
}

func (Args) Description() string {
	return `Unofficial CLI file uploader for Overcast. Version ` + version + `
Technically it's just a wrapper around upload a form at https://overcast.fm/uploads
`
}

func printf(format string, a ...interface{}) (int, error) {
	return fmt.Fprintf(outputStream, format, a...)
}

func migrateToKeyring() {
	configDirs := configdir.New("", appName)
	configDir := configDirs.QueryFolders(configdir.Global)[0]

	if !configDir.Exists("config.json") {
		return
	}
	data, err := configDir.ReadFile("config.json")
	if err != nil {
		printf("[WARN] Failed to read config file: %s\n", err)
		return
	}

	var cfg AuthData
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		printf("[WARN] Invalid JSON config file\n")
		return
	}

	err = keyring.Set(appName, "creds", string(data))
	if err != nil {
		printf("[WARN] Failed to save credentials: %s\n", err)
	}
	os.RemoveAll(configDir.Path)
}

func loadCreds() {
	migrateToKeyring()

	data, err := keyring.Get(appName, "creds")
	if err != nil {
		if err != keyring.ErrNotFound {
			printf("[WARN] Failed to load credentials: %s\n", err)
		}
		return
	}
	authDataT, err := NewAuthData([]byte(data))
	if err != nil {
		printf("[WARN] Failed to load credentials: %s\n", err)
		return
	}
	authData = *authDataT
	client.Jar.SetCookies(overcastURL, authData.GetCookies())
}

func saveCreds() {
	authData.SetCookies(client.Jar.Cookies(overcastURL))
	data, err := authData.Marshal()
	if err != nil {
		return
	}

	err = keyring.Set(appName, "creds", string(data))
	if err != nil {
		printf("[WARN] Failed to save credentials: %s\n", err)
		return
	}
}

var outputStream *os.File

func main() {
	var err error
	var args Args

	arg.MustParse(&args)

	if args.MaxParallel < 1 {
		printf("[ERROR] --parallel-uploads should be at least 1")
		os.Exit(-1)
	}

	if args.Silent {
		outputStream, err = os.Open(os.DevNull)
		// sometimes i hate golang's insistance on errorchecking
		if err != nil {
			fmt.Println("Can't open null output, your os is broken:/")
			os.Exit(-1)
		}
	} else {
		outputStream = os.Stdout
	}

	overcastURL, err = url.Parse("https://overcast.fm/")
	if err != nil {
		printf("[ERROR] %s", err)
		os.Exit(-1)
	}

	loadCreds()

	err = auth(args.Silent)
	if err != nil {
		printf("[ERROR] Auth failed: %s", err)
		os.Exit(-1)
	}

	allowedExts := []string{
		".wav",
		".mp3",
		".m4a",
		".m4b",
		".aac",
	}

	allowedExtsMap := make(map[string]struct{})
	for _, k := range allowedExts {
		allowedExtsMap[k] = struct{}{}
	}

	var totalSize int64
	var jobs []*Job

	for _, file := range args.Files {
		ext := strings.ToLower(filepath.Ext(file))
		_, found := allowedExtsMap[ext]
		if !found {
			printf("[WARN] File \"%s\" is not allowed. Allowed extentions: %s\n", file, strings.Join(allowedExts, ", "))
			continue
		}

		stat, err := os.Stat(file)
		if err != nil {
			if os.IsNotExist(err) {
				printf("[WARN] File \"%s\" doesn't exist\n", file)
			} else {
				printf("[WARN] Error with file \"%s\": %s\n", file, err)
			}
			continue
		}

		if stat.Size() > overcastParams.MaxFileSize {
			printf("[WARN] File \"%s\" is too large, max size %.2f GB\n", file, float64(overcastParams.MaxFileSize)/1000000000)
			continue
		}

		totalSize += stat.Size()

		jobs = append(jobs, NewJob(file, stat.Size()))
	}

	if totalSize > overcastParams.SpaceAvailible {
		printf("[WARN] Files are too large, you have %.2f GB availible\n", float64(overcastParams.SpaceAvailible)/1000000000)
		return
	}

	if len(jobs) > overcastParams.MaxFileCount {
		printf("[WARN] You've chosen too many files, you have %d files remaining\n", overcastParams.MaxFileCount)
		return
	}

	if len(jobs) == 0 {
		printf("[WARN] No files to upload!\n")
		return
	}

	if args.SaveCreds && authData.Changed() {
		saveCreds()
	}

	performUpload(jobs, args.MaxParallel)
}
