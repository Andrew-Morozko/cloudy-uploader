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
)

const (
	appName   = "cloudyuploader"
	version   = "1.0.0-alpha"
	appURL    = "https://github.com/Andrew-Morozko/cloudy-uploader"
	userAgent = appName + "/" + version + " CLI Uploader; " + appURL
)

var overcastURL *url.URL

var args Args
var config Config
var debug = false
var configDir *configdir.Config

type Args struct {
	Files         []string `arg:"--file,positional,required" help:"files to be uploaded"`
	MaxParallel   int      `arg:"-j,--parallel-uploads" help:"maximum number of concurrent upload jobs"`
	Login         string   `help:"email for Overcast account"`
	Password      string   `help:"password for Overcast account"`
	StoreCookie   bool     `arg:"--store-cookie" help:"store cookie to skip authorization"`
	StorePassword bool     `arg:"--store-password" help:"store (unencrypted) email/password [default: false]"`
	Silent        bool     `arg:"-s" help:"disable user interaction"`
}

func (Args) Description() string {
	return `Unofficial CLI file uploader for Overcast. Version ` + version + `
Technically it's just a wrapper around upload a form at https://overcast.fm/uploads
`
}

func printf(format string, a ...interface{}) (int, error) {
	return fmt.Fprintf(outputStream, format, a...)
}

func configLoad() {
	data, err := configDir.ReadFile("config.json")
	if err != nil {
		return
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		printf("WARN: Invalid JSON config file\n")
	}

	if len(config.Cookies) != 0 {
		client.Jar.SetCookies(overcastURL, config.GetCookies())
	}
}

func configSave() {
	configToSave := &Config{}

	if args.StoreCookie {
		configToSave.SetCookies(client.Jar.Cookies(overcastURL))
	}
	if args.StorePassword {
		configToSave.Creds = config.Creds
	}

	data, err := json.Marshal(configToSave)
	if err != nil {
		printf("[WARN] Failed to save config: %s", err.Error())
		os.Exit(-1)
	}
	err = configDir.WriteFile("config.json", data)
	if err != nil {
		printf("[WARN] Failed to save config: %s", err.Error())
		os.Exit(-1)
	}
}

var outputStream *os.File

func main() {
	var err error

	// setup default args values
	args.StoreCookie = true
	args.StorePassword = false
	args.MaxParallel = 4

	arg.MustParse(&args)

	if args.MaxParallel < 1 {
		printf("[ERROR] --parallel-uploads should be at least 1")
		os.Exit(-1)
	}

	if args.Silent {
		outputStream, _ = os.Open(os.DevNull)
	} else {
		outputStream = os.Stdout
	}

	overcastURL, err = url.Parse("https://overcast.fm/")
	if err != nil {
		printf("[ERROR] %s", err)
		os.Exit(-1)
	}

	// setup config
	configDirs := configdir.New("", appName)
	folders := configDirs.QueryFolders(configdir.Global)
	configDir = folders[0]

	configLoad()

	err = auth()
	if err != nil {
		printf("[ERROR] Auth failed: %s", err)
		os.Exit(-1)
	}

	defer configSave()

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

	performUpload(jobs)
	return
}
