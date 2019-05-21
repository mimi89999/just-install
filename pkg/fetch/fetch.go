// just-install - The simple package installer for Windows
// Copyright (C) 2019 just-install authors.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, version 3 of the License.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package fetch

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/ungerik/go-dry"
	"gopkg.in/cheggaaa/pb.v1"
)

// Options that influence Fetch.
type Options struct {
	Destination string // Can either be a file path or a directory path. If it's a directory, it must already exist.
	Progress    bool   // Whether to show the progress indicator.
}

// Fetch obtains the given resource, either a local file or something that can be download via
// HTTP/HTTPS, to a file on disk. Returns the path to the fetched file or an error.
func Fetch(resource string, options *Options) (string, error) {
	// Shortcut: resource is a local file and we can return its path immediately.
	if dry.FileExists(resource) {
		return resource, nil
	}

	// Options
	if options == nil {
		options = &Options{}
	}

	if options.Destination == "" {
		return "", errors.New("destination must be either a file or directory path")
	}

	// Parse resource URL
	parsedURL, err := url.Parse(resource)
	if err != nil {
		return "", err
	}

	switch parsedURL.Scheme {
	case "file":
		return parsedURL.Path, nil
	case "http":
		fallthrough
	case "https":
		return fetchHTTP(parsedURL, options)
	default:
		return "", fmt.Errorf("unknown URL scheme: %s", parsedURL.Scheme)
	}
}

// fetchHTTP downloads the given file via HTTP or HTTPS.
func fetchHTTP(resource *url.URL, options *Options) (string, error) {
	// Options
	if options == nil {
		options = &Options{}
	}

	// Request
	req, err := http.NewRequest("GET", resource.String(), nil)
	if err != nil {
		return "", err
	}

	httpClient := NewClient()

	var lastLocation *url.URL
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// This is the same check used by the CheckRedirect function used in the standard library.
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}

		lastLocation = req.URL
		return nil
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("expected 200 instead got %v at %v", resp.StatusCode, resource)
	}

	// Compute final destination path
	dest := options.Destination
	if dry.FileIsDir(dest) {
		if lastLocation == nil {
			dest = filepath.Join(dest, filepath.Base(resource.Path))
		} else {
			dest = filepath.Join(dest, filepath.Base(lastLocation.Path))
		}
	}

	// File already exists, return its path.
	if dry.FileExists(dest) {
		return dest, nil
	}

	// Fetch to temporary file
	destTmp := dest + ".download"

	destTmpWriter, err := os.Create(destTmp)
	if err != nil {
		return "", err
	}
	defer destTmpWriter.Close()

	var copyWriter io.Writer = destTmpWriter
	if options.Progress {
		progressBar := pb.New64(resp.ContentLength)
		progressBar.ShowSpeed = true
		progressBar.SetRefreshRate(time.Millisecond * 1000)
		progressBar.SetUnits(pb.U_BYTES)
		progressBar.Start()
		defer progressBar.Finish()

		copyWriter = io.MultiWriter(destTmpWriter, progressBar)
	}

	if _, err := io.Copy(copyWriter, resp.Body); err != nil {
		return "", err
	}

	destTmpWriter.Close()
	resp.Body.Close()

	// Move temporary file back to definitive place
	if err := os.Rename(destTmp, dest); err != nil {
		return "", err
	}

	return dest, nil
}
