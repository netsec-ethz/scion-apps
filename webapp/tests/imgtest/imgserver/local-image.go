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

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func main() {
	for {
		t := time.Now()
		filename := fmt.Sprintf("remote-%04d%02d%02d-%02d:%02d:%02d.jpg",
			t.Year(), t.Month(), t.Day(),
			t.Hour(), t.Minute(), t.Second())
		out, err := os.Create(filename)
		if err != nil {
			log.Println("os.Create() error: " + err.Error())
		}

		// generate random light-colored image
		img := image.NewRGBA(image.Rect(0, 0, 250, 250))
		rand.Seed(t.UnixNano())
		rr := uint8(rand.Intn(127) + 127)
		rg := uint8(rand.Intn(127) + 127)
		rb := uint8(rand.Intn(127) + 127)
		color := color.RGBA{rr, rg, rb, 255}
		draw.Draw(img, img.Bounds(), &image.Uniform{color}, image.ZP, draw.Src)

		// add time to img
		x, y := 5, 100
		addImgLabel(img, x, y, t.Format(time.RFC850))

		// add hostname to img
		name, err := os.Hostname()
		if err != nil {
			log.Println("os.Hostname() error: " + err.Error())
		}
		y += 20
		addImgLabel(img, x, y, name)

		// add address to img
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			log.Println("net.InterfaceAddrs() error: " + err.Error())
		}
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					y += 20
					addrStr := fmt.Sprintf("%s (%s)", ipnet.IP.String(), a.Network())
					addImgLabel(img, x, y, addrStr)
				}
			}
		}

		// add cpu utilization to img
		idle0, total0 := getCPUSample()
		time.Sleep(3 * time.Second)
		idle1, total1 := getCPUSample()
		idleTicks := float64(idle1 - idle0)
		totalTicks := float64(total1 - total0)
		cpuUsage := 100 * (totalTicks - idleTicks) / totalTicks
		cpu := fmt.Sprintf("CPU utilization %.1f%%", cpuUsage)
		y += 20
		addImgLabel(img, x, y, cpu)

		var opt jpeg.Options
		opt.Quality = 100
		err = jpeg.Encode(out, img, &opt)
		if err != nil {
			log.Println("jpeg.Encode() error: " + err.Error())
		}

		time.Sleep(120 * time.Second)
	}
}

// Queries the linux system status /proc/stat and parses stats.
// From bertimus9 at stackoverflow.com/q/11356330.
func getCPUSample() (idle, total uint64) {
	contents, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if fields[0] == "cpu" {
			numFields := len(fields)
			for i := 1; i < numFields; i++ {
				val, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					fmt.Println("Error: ", i, fields[i], err)
				}
				total += val // tally up all the numbers to get total ticks
				if i == 4 {  // idle is the 5th field in the cpu line
					idle = val
				}
			}
			return
		}
	}
	return
}

// Configures font to render label at x, y on the img.
func addImgLabel(img *image.RGBA, x, y int, label string) {
	col := color.RGBA{0, 0, 0, 255}
	point := fixed.Point26_6{fixed.Int26_6(x * 64), fixed.Int26_6(y * 64)}
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  point,
	}
	d.DrawString(label)
}
