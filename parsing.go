package main

import (
	"fmt"
	"io"
	"regexp"
	"strconv"

	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
)

// Parsing the stuff

type OvercastParams struct {
	SpaceAvailible int64
	MaxFileCount   int
	MaxFileSize    int64
	PostData       map[string]string
	UploadURL      string
	DataKeyPrefix  string
}

// extracts limitations from the /uploads page
func parseInfo(input *goquery.Selection) (avalible int64, maxFiles int, maxFile int64) {
	var err error
	avalibleStr, found := input.Attr("data-free-bytes")
	if found {
		avalible, err = strconv.ParseInt(avalibleStr, 10, 64)
	}
	if err != nil || !found {
		avalible = -1
		fmt.Println("[WARN] Failed to get space limit, upload might fail")
	}

	maxFileStr, found := input.Attr("data-max-bytes")
	if found {
		maxFile, err = strconv.ParseInt(maxFileStr, 10, 64)
	}
	if err != nil || !found {
		maxFile = -1
		fmt.Println("[WARN] Failed to get file size limit, upload might fail")
	}

	maxFiles = -1

	info := input.NextFiltered("div.caption2").Text()

	reMaxFiles := regexp.MustCompile(`up\s+to\s+(\d+)`)

	maxFilesStrs := reMaxFiles.FindStringSubmatch(info)

	if len(maxFilesStrs) == 2 {
		maxFiles, err = strconv.Atoi(maxFilesStrs[1])
		if err != nil {
			maxFiles = -1
		}
	}

	if maxFile == -1 {
		fmt.Println("[WARN] Failed to get total files limit, upload might fail")
	}

	return
}

func parseUploadsPage(body io.ReadCloser) (params *OvercastParams, err error) {
	var overcastParams OvercastParams
	uploadsPage, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		err = errors.Wrap(err, "Error while parsing /uploads page")
		return
	}

	form := uploadsPage.Find("form#upload_form")

	prefix, found := form.Attr("data-key-prefix")
	if !found {
		err = errors.New("Failed to parse upload form: no data-key-prefix found")
		return
	}
	overcastParams.DataKeyPrefix = prefix
	overcastParams.PostData = make(map[string]string)

	form.Find(`input[type="hidden"]`).Each(func(i int, s *goquery.Selection) {
		name, nameFound := s.Attr("name")
		val, valueFound := s.Attr("value")
		if nameFound && valueFound {
			overcastParams.PostData[name] = val
		}
	})

	uploadURL, uploadURLFound := form.Attr("action")

	if form.Length() != 1 || len(overcastParams.PostData) == 0 || !uploadURLFound {
		err = errors.New("Failed to find the upload form")
		return
	}

	overcastParams.UploadURL = uploadURL

	input := uploadsPage.Find("input#upload_file")

	overcastParams.SpaceAvailible, overcastParams.MaxFileCount, overcastParams.MaxFileSize = parseInfo(input)

	return &overcastParams, nil
}
