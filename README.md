# Download - File Download, support parallel

[![PkgGoDev](https://pkg.go.dev/badge/github.com/go-zoox/download)](https://pkg.go.dev/github.com/go-zoox/download)
[![Build Status](https://github.com/go-zoox/download/actions/workflows/ci.yml/badge.svg?branch=master)](https://github.com/go-zoox/download/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-zoox/download)](https://goreportcard.com/report/github.com/go-zoox/download)
[![Coverage Status](https://coveralls.io/repos/github/go-zoox/download/badge.svg?branch=master)](https://coveralls.io/github/go-zoox/download?branch=master)
[![GitHub issues](https://img.shields.io/github/issues/go-zoox/download.svg)](https://github.com/go-zoox/download/issues)
[![Release](https://img.shields.io/github/tag/go-zoox/download.svg?label=Release)](https://github.com/go-zoox/download/tags)

## Installation
To install the package, run:
```bash
go get github.com/go-zoox/download
```

## Getting Started

```go
func TestDownload(t *testing.T) {
	url := "YOUR_FILE_URL"
	fileName := "test.mp4"
	err := Download(url, &Config{
		FilePath: fileName,
	})
	if err != nil {
		t.Error(err)
	}
}
```

## Functions
* [x] Parallel
* [ ] Progress

## License
GoZoox is released under the [MIT License](./LICENSE).