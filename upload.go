package main

import (
	"bytes"

	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Andrew-Morozko/cloudy-uploader/mbpdecor"

	"github.com/pkg/errors"
	"github.com/vbauerster/mpb/v4"
	"github.com/vbauerster/mpb/v4/decor"
)

type Job struct {
	File             string
	FileName         string
	FileSize         int64
	ProgressBars     []*mpb.Bar
	status           *mbpdecor.StatusDecorator
	isDone           bool
	amazonUploadDone chan struct{}
}

func NewJob(file string, filesize int64) *Job {
	return &Job{
		File:             file,
		FileName:         filepath.Base(file),
		FileSize:         filesize,
		amazonUploadDone: make(chan struct{}),
	}
}

func (job *Job) setEndState(msg string) {
	job.isDone = true
	job.status.SetStatus(msg)
	for _, bar := range job.ProgressBars {
		if !bar.Completed() {
			bar.SetTotal(0, true)
		}
	}
}

func (job *Job) Done() {
	job.setEndState("Uploaded!")
}

func (job *Job) SetError(msg string) {
	job.setEndState("Error: " + msg)
}

func (job *Job) BeginUpload(totalSize int64) {
	job.ProgressBars[1].SetTotal(totalSize, false)
	job.ProgressBars[0].SetTotal(0, true)
}
func (job *Job) GetUploadReader(reader io.Reader) io.ReadCloser {
	return job.ProgressBars[1].ProxyReader(reader)
}

func performUpload(jobs []*Job, maxParallel int, unorderedSubmit bool, overcastParams *OvercastParams) {
	bars := mpb.New()

	var bar *mpb.Bar
	for _, job := range jobs {
		// setting up bar progression for this job
		jobTitle := strings.TrimSuffix(job.FileName, filepath.Ext(job.FileName)) + ":"

		bar = bars.AddSpinner(1,
			mpb.SpinnerOnLeft,
			mpb.PrependDecorators(
				decor.Name(jobTitle, decor.WCSyncSpaceR),
				decor.Merge(
					decor.Name("Waiting", decor.WCSyncWidthR),
					decor.WCSyncWidthR,
					decor.WCSyncWidth,
				),
			),
			mpb.AppendDecorators(
				decor.Name(
					fmt.Sprintf("% .1f", decor.SizeB1000(job.FileSize)),
					decor.WCSyncWidth,
				),
			),
		)
		job.ProgressBars = append(job.ProgressBars, bar)

		bar = bars.AddBar(1, // Will be set later, after the total request length is calculated
			mpb.BarParkTo(bar),
			mpb.PrependDecorators(
				decor.Name(jobTitle, decor.WCSyncSpaceR),
				decor.Name("Uploading @ ", decor.WCSyncWidthR),
				decor.AverageSpeed(decor.UnitKB, "% .2f", decor.WCSyncWidth),
			),
			mpb.AppendDecorators(
				decor.CountersKiloByte("% .1f / % .1f", decor.WCSyncWidth),
				decor.Name("ETA ", decor.WCSyncSpace),
				decor.EwmaETA(decor.ET_STYLE_MMSS, 90.0, decor.WCSyncSpaceR),
			),
		)
		job.ProgressBars = append(job.ProgressBars, bar)

		job.status = mbpdecor.Status("Waiting", decor.WCSyncWidthR)

		bar = bars.AddSpinner(1,
			mpb.SpinnerOnLeft,
			mpb.BarParkTo(bar),
			mpb.BarClearOnComplete(),
			mpb.PrependDecorators(
				decor.Name(jobTitle, decor.WCSyncSpaceR),
				decor.Merge(
					job.status,
					decor.WCSyncWidthR,
					decor.WCSyncWidth,
				),
			),
		)
		job.ProgressBars = append(job.ProgressBars, bar)
	}

	amazonUploadPermissionC := make(chan struct{}, maxParallel)
	for i := 0; i < maxParallel; i++ {
		amazonUploadPermissionC <- struct{}{}
	}
	go func() {
		for _, job := range jobs {
			<-amazonUploadPermissionC
			go func(job *Job) {
				err := uploadToAmazon(job, overcastParams.UploadURL, overcastParams.PostData)
				amazonUploadPermissionC <- struct{}{}
				if err != nil {
					job.SetError(err.Error())
				}
				close(job.amazonUploadDone)
				if unorderedSubmit {
					job.status.SetStatus("Submitting")
					err := submitToOvercast(job, overcastParams.DataKeyPrefix)
					if err != nil {
						job.SetError(err.Error())
					} else {
						job.Done()
					}
				}
			}(job)
		}
	}()

	if !unorderedSubmit {
		overcastSubmitPermissionC := make(chan struct{}, 1)
		overcastSubmitPermissionC <- struct{}{}
		overcastSubmitDelay := 2 * time.Second

		for _, job := range jobs {
			<-job.amazonUploadDone
			if job.isDone {
				continue
			}

			<-overcastSubmitPermissionC
			job.status.SetStatus("Submitting")
			err := submitToOvercast(job, overcastParams.DataKeyPrefix)
			if err != nil {
				job.SetError(err.Error())
				overcastSubmitPermissionC <- struct{}{}
			} else {
				job.Done()
				time.AfterFunc(overcastSubmitDelay, func() {
					overcastSubmitPermissionC <- struct{}{}
				})
			}
		}
	}

	bars.Wait()
}

func dumpBytesFromBuf(byteBuf *bytes.Buffer) ([]byte, error) {
	array := make([]byte, byteBuf.Len())
	_, err := byteBuf.Read(array)
	if err != nil {
		return nil, err
	}
	return array, nil
}

func uploadToAmazon(job *Job, uploadUrl string, postData map[string]string) (err error) {
	//buffer for storing multipart data
	byteBuf := &bytes.Buffer{}

	//part: parameters
	mpWriter := multipart.NewWriter(byteBuf)

	for key, value := range postData {
		err = mpWriter.WriteField(key, value)
		if err != nil {
			return
		}
	}

	_, err = mpWriter.CreateFormFile("file", job.FileName)
	if err != nil {
		return
	}
	multipartStart, err := dumpBytesFromBuf(byteBuf)
	if err != nil {
		return
	}
	err = mpWriter.Close()
	if err != nil {
		return
	}
	multipartEnd, err := dumpBytesFromBuf(byteBuf)
	if err != nil {
		return
	}

	//calculate content length
	totalSize := int64(len(multipartStart)) + job.FileSize + int64(len(multipartEnd))
	job.BeginUpload(totalSize)

	file, err := os.Open(job.File)
	if err != nil {
		return
	}
	defer file.Close()

	comboReader := io.MultiReader(
		bytes.NewReader(multipartStart),
		io.LimitReader(file, job.FileSize), // Just in case the file would be modified while uploading
		bytes.NewReader(multipartEnd),
	)

	req, err := http.NewRequest("POST", uploadUrl, job.GetUploadReader(comboReader))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", mpWriter.FormDataContentType())
	req.Header.Set("Origin", "https://overcast.fm")
	req.ContentLength = totalSize

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()

	if resp.StatusCode != 204 {
		return errors.Errorf("Unexpected status code from amazon: %d", resp.StatusCode)
	}
	return
}

func submitToOvercast(job *Job, dataKeyPrefix string) (err error) {
	byteBuf := &bytes.Buffer{}
	mpWriter := multipart.NewWriter(byteBuf)

	amazonKey := dataKeyPrefix + job.FileName
	err = mpWriter.WriteField("key", amazonKey)
	if err != nil {
		return
	}
	err = mpWriter.Close()
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", "https://overcast.fm/podcasts/upload_succeeded", byteBuf)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", mpWriter.FormDataContentType())
	req.Header.Set("Origin", "https://overcast.fm")
	req.ContentLength = int64(byteBuf.Len())

	resp, err := client.Do(req)
	if err != nil {
		return
	}

	if resp.StatusCode != 200 {
		return errors.New("Unexpected status code from overcast")
	}

	err = resp.Body.Close()
	if err != nil {
		return
	}

	return
}
