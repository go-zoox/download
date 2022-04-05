package download

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-zoox/crypto/md5"
	"github.com/go-zoox/fetch"
	"github.com/go-zoox/fs"
)

// DefaultSegmentSize stands for the default segment size (10 Mb)
//	if the segment size is not set, the default segment size is used
var DefaultSegmentSize = 10 * 1024 * 1024

// Downloader is the downloader
type Downloader struct {
	// URL is the url to download
	URL string
	// FileDir represents the directory to store the downloaded file
	FileDir string
	// FileName represents the file name
	FileName string
	// FileExt represents the file extension
	FileExt string
	// HeadHeaders represents the headers of the head file response
	HeadHeaders http.Header
	// ContentType represents the content type of the file
	ContentType string
	// ContentLength represents the content length of the file
	ContentLength int64
	// Hash represents the file info hash, use for temp dir
	Hash string
	// IsSupportRange represents if the server supports the range header
	IsSupportRange bool
	// SegmentSize represents the size of each segment, default is 10 Mb
	SegmentSize int
	// Ranges represents the ranges of the file
	Ranges []*Range
	// FileParts represents the file parts by ranges
	FileParts []*FilePart
	// TmpDir represents the temporary directory to store file parts
	TmpDir string
	//
	IsRangesDisabled bool
}

// Range represents the range of the file
type Range struct {
	Start int
	End   int
}

// FilePart represents a part of a file.
// Format: <file_name>.<file_ext>.part.<part_index>.<range_start>.<range_end>
type FilePart struct {
	// Name of the file.
	// format <file_name>.<file_ext>.part.<part_index>.<range_start>.<range_end>
	Name string
	// FilePath concats the TmpDir and Hash.
	Path string
	//
	FileName   string
	FileExt    string
	Index      int
	RangeStart int
	RangeEnd   int
}

// Config represents the download config
type Config struct {
	// FileName
	FilePath string
	// SegmentSize
	SegmentSize int
	// TmpDir
	TmpDir string
	//
	IsRangesDisabled bool
}

// New returns a new downloader
func New(url string, config *Config) *Downloader {
	SegmentSize := DefaultSegmentSize
	TmpDir := fs.TmpDirPath()
	FileDir := fs.CurrentDir()
	FileName := ""
	FileExt := ""
	IsRangesDisabled := false
	if config.SegmentSize > 0 {
		SegmentSize = config.SegmentSize
	}
	if config.TmpDir != "" {
		TmpDir = config.TmpDir
	}
	if config.FilePath != "" {
		FileDir = fs.DirName(config.FilePath)
		paths := strings.Split(config.FilePath, "/")
		last := paths[len(paths)-1]
		exts := strings.Split(last, ".")
		if len(exts) > 1 {
			FileName = strings.Join(exts[:len(exts)-1], ".")
			FileExt = exts[len(exts)-1]
		} else {
			FileName = last
		}
	}
	if config.IsRangesDisabled {
		IsRangesDisabled = config.IsRangesDisabled
	}

	return &Downloader{
		URL:              url,
		SegmentSize:      SegmentSize,
		TmpDir:           TmpDir,
		FileDir:          FileDir,
		FileName:         FileName,
		FileExt:          FileExt,
		IsRangesDisabled: IsRangesDisabled,
	}
}

func (d *Downloader) jsonify(i interface{}) (string, error) {
	b, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func (d *Downloader) printJSON(i interface{}) {
	fmt.Println(d.jsonify(i))
}

func (d *Downloader) getFilePath() string {
	if d.FileName == "" {
		return ""
	}

	return fmt.Sprintf("%s/%s.%s", d.FileDir, d.FileName, d.FileExt)
}

func (d *Downloader) parseURL(u string) error {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return errors.New("invalid url: " + u + ": " + err.Error())
	}

	if d.FileName == "" {
		paths := strings.Split(parsedURL.Path, "/")
		last := paths[len(paths)-1]
		exts := strings.Split(last, ".")
		if len(exts) > 1 {
			d.FileName = strings.Join(exts[:len(exts)-1], ".")
			d.FileExt = exts[len(exts)-1]
		} else {
			d.FileName = last
		}
	}

	return nil
}

func (d *Downloader) parseContentInfo() error {
	headers := d.HeadHeaders

	// 1. content type
	d.ContentType = headers.Get("Content-Type")

	// 2. content length
	contentLengthRaw := headers.Get("Content-Length")
	contentLengthInt, _ := strconv.Atoi(contentLengthRaw)
	if contentLengthInt > 0 {
		d.ContentLength = int64(contentLengthInt)
	}

	return nil
}

func (d *Downloader) parseRanges() error {
	// 3. ranges
	if d.ContentLength > 0 {
		start := 0
		end := int(d.ContentLength - 1)
		for {
			if start+d.SegmentSize > end {
				d.Ranges = append(d.Ranges, &Range{
					Start: start,
					End:   end,
				})

				break
			}

			d.Ranges = append(d.Ranges, &Range{
				Start: start,
				End:   start + d.SegmentSize - 1,
			})

			start += d.SegmentSize
		}
	}

	return nil
}

func (d *Downloader) parseFileParts() error {
	if len(d.Ranges) == 0 {
		return nil
	}

	for i, r := range d.Ranges {
		// Name := fmt.Sprintf("%s.%s.part.%d.%d.%d", d.FileName, d.FileExt, i, r.Start, r.End)
		Name := fmt.Sprintf("part.%d.%d.%d", i, r.Start, r.End)
		Path := fs.JoinPath(d.TmpDir, d.Hash, Name)
		filePart := &FilePart{
			Name:       Name,
			Path:       Path,
			FileName:   d.FileName,
			FileExt:    d.FileExt,
			Index:      i,
			RangeStart: r.Start,
			RangeEnd:   r.End,
		}

		d.FileParts = append(d.FileParts, filePart)
	}

	return nil
}

func (d *Downloader) parseFileInfo() error {
	if d.FileExt == "" {
		if d.ContentType == "video/mp4" {
			d.FileExt = "mp4"
		} else if d.ContentType == "video/webm" {
			d.FileExt = "webm"
		} else if d.ContentType == "video/ogg" {
			d.FileExt = "ogg"
		} else if d.ContentType == "video/x-flv" {
			d.FileExt = "flv"
		} else if d.ContentType == "video/x-ms-wmv" {
			d.FileExt = "wmv"
		} else if d.ContentType == "video/x-msvideo" {
			d.FileExt = "avi"
		} else if d.ContentType == "video/x-matroska" {
			d.FileExt = "mkv"
		} else if d.ContentType == "video/mpeg" {
			d.FileExt = "mpg"
		} else if d.ContentType == "video/quicktime" {
			d.FileExt = "mov"
		} else if d.ContentType == "video/x-ms-asf" {
			d.FileExt = "asf"
		} else if d.ContentType == "video/x-ms-wm" {
			d.FileExt = "wm"
		} else if d.ContentType == "video/x-ms-wmx" {
			d.FileExt = "wmx"
		} else if d.ContentType == "video/x-ms-wvx" {
			d.FileExt = "wvx"
		} else if d.ContentType == "video/x-ms-wax" {
			d.FileExt = "wax"
		} else if d.ContentType == "audio/mpeg" {
			d.FileExt = "mp3"
		} else if d.ContentType == "audio/x-ms-wma" {
			d.FileExt = "wma"
		} else {
			return errors.New("unsupported content type: " + d.ContentType)
		}
	}

	return nil
}

func (d *Downloader) parseHash() error {
	data := []string{
		d.URL,
		d.ContentType,
		strconv.FormatInt(d.ContentLength, 10),
		// d.FileName,
		// d.FileExt,
	}

	d.Hash = md5.Md5(strings.Join(data, "-"))
	return nil
}

func (d *Downloader) parse() error {
	if err := d.parseContentInfo(); err != nil {
		return err
	}

	if err := d.parseRanges(); err != nil {
		return err
	}

	if err := d.parseFileInfo(); err != nil {
		return err
	}

	if err := d.parseHash(); err != nil {
		return err
	}

	if err := d.parseFileParts(); err != nil {
		return err
	}

	return nil
}

func (d *Downloader) checkSupportRange() (bool, error) {
	response, err := fetch.Head(d.URL)
	if err != nil {
		return d.IsSupportRange, err
	}

	if response.Headers.Get("Accept-Ranges") == "bytes" {
		d.IsSupportRange = true
		d.HeadHeaders = response.Headers.Clone()
		return d.IsSupportRange, nil
	}

	return d.IsSupportRange, nil
}

func (d *Downloader) downloadFilePart(part *FilePart) error {
	// 1. check file part
	if fs.IsExist(part.Path) {
		if fs.Size(part.Path) == int64(part.RangeEnd-part.RangeStart+1) {
			return nil
		}
	}

	//
	dirPath := fs.DirName(part.Path)
	if !fs.IsExist(dirPath) {
		if err := fs.Mkdir(dirPath); err != nil {
			return err
		}
	}

	// 2. download file part
	response, err := fetch.Get(d.URL, &fetch.Config{
		Headers: map[string]string{
			"Range": fmt.Sprintf("bytes=%d-%d", part.RangeStart, part.RangeEnd),
		},
		Timeout: 120 * time.Second,
	})
	if err != nil {
		return err
	}

	// Valid
	// Content-Range: bytes 0-10485759/35519965
	contentRangeRaw := response.Headers.Get("Content-Range")
	if contentRangeRaw == "" {
		return errors.New("no content range")
	}
	contentRangeParts := strings.Split(contentRangeRaw, " ")
	if len(contentRangeParts) != 2 {
		return errors.New("invalid content range (1): bytes")
	}
	contentRangeParts = strings.Split(contentRangeParts[1], "/")
	if len(contentRangeParts) != 2 {
		return errors.New("invalid content range (2): range/total")
	}
	if contentRangeParts[0] != fmt.Sprintf("%d-%d", part.RangeStart, part.RangeEnd) {
		return errors.New("invalid content range (3): range error")
	}
	// Content-Length: 35519965
	contentLength, err := strconv.Atoi(response.Headers.Get("Content-Length"))
	if err != nil {
		return err
	}
	if contentLength != part.RangeEnd-part.RangeStart+1 {
		return errors.New("invalid content length")
	}

	// d.printJSON(map[string]interface{}{
	// 	"url":   d.Url,
	// 	"Range": fmt.Sprintf("bytes=%d-%d", part.RangeStart, part.RangeEnd),
	// })
	// d.printJSON(response.Headers)
	// os.Exit(1)

	if response.Status != http.StatusPartialContent {
		return fmt.Errorf("invalid status: %d", response.Status)
	}

	if err := fs.WriteFile(part.Path, response.Body); err != nil {
		return err
	}

	return nil
}

func (d *Downloader) downloadFileParts() (err error) {
	wg := sync.WaitGroup{}
	wg.Add(len(d.FileParts))

	for _, part := range d.FileParts {
		go func(part *FilePart) {
			defer wg.Done()

			if os.Getenv("DEBUG") == "true" {
				fmt.Println("downloading part :", part.Index, part.Path)
			}

			err = d.downloadFilePart(part)
		}(part)
	}

	wg.Wait()
	return
}

func (d *Downloader) mergeFileParts() error {
	parts := d.FileParts
	filePath := d.getFilePath()

	_parts := make([]*fs.FilePart, 0)
	for _, part := range parts {
		_parts = append(_parts, &fs.FilePart{
			Path:  part.Path,
			Index: part.Index,
		})
	}

	return fs.Merge(filePath, _parts)
}

func (d *Downloader) downloadByRanges() error {
	// 1. Check server support range.
	isSupportRange, err := d.checkSupportRange()
	if err != nil {
		return err
	}

	if !isSupportRange {
		return errors.New("server does not support range")
	}

	// 2. Parse file info.
	err = d.parse()
	if err != nil {
		return err
	}

	if os.Getenv("DEBUG") == "true" {
		d.printJSON(d)
	}

	// 2. Download file.
	if err := d.downloadFileParts(); err != nil {
		return err
	}

	if err := d.mergeFileParts(); err != nil {
		return err
	}

	return nil
}

func (d *Downloader) downloadByDirect() error {
	response, err := fetch.Get(d.URL)
	if err != nil {
		return err
	}

	if err := fs.WriteFile(d.getFilePath(), response.Body); err != nil {
		return err
	}

	return nil
}

// Download downloads the file
func (d *Downloader) Download() error {
	// parse url get file info
	err := d.parseURL(d.URL)
	if err != nil {
		return err
	}

	// download directory
	if d.IsRangesDisabled {
		return d.downloadByDirect()
	}

	// download with ranges
	return d.downloadByRanges()
}

// Download downloads the file by url and config
func Download(url string, cfg ...*Config) error {
	configX := &Config{}
	if len(cfg) > 0 {
		configX = cfg[0]
	}

	d := New(url, configX)
	return d.Download()
}
