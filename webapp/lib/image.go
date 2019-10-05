// Copyright 2019 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.package main

package lib

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"

	log "github.com/inconshreveable/log15"
	. "github.com/netsec-ethz/scion-apps/webapp/util"
)

var regexImageFiles = `([^\s]+(\.(?i)(jp?g|png|gif))$)`

var imgTemplate = `<!doctype html><html lang="en"><head></head><body>
<a href="{{.ImgUrl}}" target="_blank"><img src="data:image/jpg;base64,{{.JpegB64}}">
</a></body>`

// Handles writing jpeg image to http response writer by content-type.
func writeJpegContentType(w http.ResponseWriter, img *image.Image) {
	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, *img, nil)
	CheckError(err)
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(buf.Bytes())))
	_, werr := w.Write(buf.Bytes())
	CheckError(werr)
}

// Handles writing jpeg image to http response writer by image template.
func writeJpegTemplate(w http.ResponseWriter, img *image.Image, url string) {
	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, *img, nil)
	CheckError(err)
	str := base64.StdEncoding.EncodeToString(buf.Bytes())
	tmpl, err := template.New("image").Parse(imgTemplate)
	if CheckError(err) {
		return
	}
	data := map[string]interface{}{"JpegB64": str, "ImgUrl": url}
	err = tmpl.Execute(w, data)
	CheckError(err)
}

// Helper method to find most recently modified regex extension filename in dir.
func findNewestFileExt(dir, extRegEx string) (imgFilename string, imgTimestamp int64) {
	files, _ := ioutil.ReadDir(dir)
	for _, f := range files {
		fi, err := os.Stat(path.Join(dir, f.Name()))
		CheckError(err)
		matched, err := regexp.MatchString(extRegEx, f.Name())
		CheckError(err)
		if matched {
			modTime := fi.ModTime().Unix()
			if modTime > imgTimestamp {
				imgTimestamp = modTime
				imgFilename = f.Name()
			}
		}
	}
	return
}

// FindImageInfoHandler locating most recent image and writing text info data about it.
func FindImageInfoHandler(w http.ResponseWriter, r *http.Request, options *CmdOptions) {
	filesDir := path.Join(options.StaticRoot, "data/images")
	imgFilename, _ := findNewestFileExt(filesDir, regexImageFiles)
	if imgFilename == "" {
		return
	}
	fileText := imgFilename
	fmt.Fprintf(w, fileText)
}

// FindImageHandler locating most recent image formatting it for graphic display in response.
func FindImageHandler(w http.ResponseWriter, r *http.Request, options *CmdOptions, browserAddr string, port int) {
	filesDir := path.Join(options.StaticRoot, "data/images")
	imgFilename, _ := findNewestFileExt(filesDir, regexImageFiles)
	if imgFilename == "" {
		fmt.Fprint(w, "Error: Unable to find image file locally.")
		return
	}
	log.Info("Found image file:", "imgFilename", imgFilename)
	imgFile, err := os.Open(path.Join(filesDir, imgFilename))
	CheckError(err)
	defer imgFile.Close()
	_, imageType, err := image.Decode(imgFile)
	CheckError(err)
	log.Info("Found image type: ", "imageType", imageType)
	// reset file pointer to beginning
	imgFile.Seek(0, 0)
	var rawImage image.Image
	switch imageType {
	case "gif":
		rawImage, err = gif.Decode(imgFile)
	case "png":
		rawImage, err = png.Decode(imgFile)
	case "jpeg":
		rawImage, err = jpeg.Decode(imgFile)
	default:
		panic("Unhandled image type!")
	}
	CheckError(err)
	url := fmt.Sprintf("http://%s:%d/%s/%s", browserAddr, port, "data/images", imgFilename)
	writeJpegTemplate(w, &rawImage, url)
}
