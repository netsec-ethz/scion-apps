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
    "log"
    "net/http"
    "os"
    "path"
    "regexp"
    "strconv"
)

var regexImageFiles = `([^\s]+(\.(?i)(jp?g|png|gif))$)`

var imgTemplate = `<!doctype html><html lang="en"><head></head><body>
<a href="{{.ImgUrl}}" target="_blank"><img src="data:image/jpg;base64,{{.JpegB64}}">
</a></body>`

// Handles writing jpeg image to http response writer by content-type.
func writeJpegContentType(w http.ResponseWriter, img *image.Image) {
    buf := new(bytes.Buffer)
    err := jpeg.Encode(buf, *img, nil)
    if err != nil {
        log.Println("jpeg.Encode() error: " + err.Error())
    }
    w.Header().Set("Content-Type", "image/jpeg")
    w.Header().Set("Content-Length", strconv.Itoa(len(buf.Bytes())))
    _, werr := w.Write(buf.Bytes())
    if werr != nil {
        log.Println("w.Write() image error: " + werr.Error())
    }
}

// Handles writing jpeg image to http response writer by image template.
func writeJpegTemplate(w http.ResponseWriter, img *image.Image, url string) {
    buf := new(bytes.Buffer)
    err := jpeg.Encode(buf, *img, nil)
    if err != nil {
        log.Println("jpeg.Encode() error: " + err.Error())
    }
    str := base64.StdEncoding.EncodeToString(buf.Bytes())
    tmpl, err := template.New("image").Parse(imgTemplate)
    if err != nil {
        log.Println("tmpl.Parse() image error: " + err.Error())
    } else {
        data := map[string]interface{}{"JpegB64": str, "ImgUrl": url}
        err := tmpl.Execute(w, data)
        if err != nil {
            log.Println("tmpl.Execute() image error: " + err.Error())
        }
    }
}

// Helper method to find most recently modified regex extension filename in dir.
func findNewestFileExt(dir, extRegEx string) (imgFilename string, imgTimestamp int64) {
    files, _ := ioutil.ReadDir(dir)
    for _, f := range files {
        fi, err := os.Stat(path.Join(dir, f.Name()))
        if err != nil {
            log.Println("os.Stat() error: " + err.Error())
        }
        matched, err := regexp.MatchString(extRegEx, f.Name())
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
func FindImageInfoHandler(w http.ResponseWriter, r *http.Request) {
    filesDir := "."
    imgFilename, _ := findNewestFileExt(filesDir, regexImageFiles)
    if imgFilename == "" {
        return
    }
    fileText := imgFilename
    fmt.Fprintf(w, fileText)
}

// FindImageHandler locating most recent image formatting it for graphic display in response.
func FindImageHandler(w http.ResponseWriter, r *http.Request, browserAddr string, port int) {
    filesDir := "."
    imgFilename, _ := findNewestFileExt(filesDir, regexImageFiles)
    if imgFilename == "" {
        fmt.Fprint(w, "Error: Unable to find image file locally.")
        return
    }
    log.Println("Found image file: " + imgFilename)
    imgFile, err := os.Open(path.Join(filesDir, imgFilename))
    if err != nil {
        log.Println("os.Open() error: " + err.Error())
    }
    defer imgFile.Close()
    _, imageType, err := image.Decode(imgFile)
    if err != nil {
        log.Println("image.Decode() error: " + err.Error())
    }
    log.Println("Found image type: " + imageType)
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
    if err != nil {
        log.Println("png.Decode() error: " + err.Error())
    }
    url := fmt.Sprintf("http://%s:%d/%s/%s", browserAddr, port, "images", imgFilename)
    writeJpegTemplate(w, &rawImage, url)
}
