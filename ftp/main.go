// Copyright 2020 ETH Zurich
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
// limitations under the License.

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/netsec-ethz/scion-apps/ftp/internal/ftp"
)

func main() {
	app := App{
		ctx:            context.Background(),
		herculesBinary: *herculesFlag,
	}

	app.cmd = commandMap{
		"help":    app.help,
		"connect": app.connect,
		"login":   app.login,
		"ls":      app.ls,
		"cd":      app.cd,
		"pwd":     app.pwd,
		"mode":    app.mode,
		"get":     app.retr,
		"put":     app.stor,
		"mkdir":   app.mkdir,
		"quit":    app.quit,
	}

	if err := app.run(); err != nil {
		fmt.Println(err)
	}
}

type commandMap map[string]func([]string)

var (
	herculesFlag = flag.String("hercules", "", "Enable Hercules mode using the Hercules binary specified\nIn Hercules mode, scionFTP checks the following directories for Hercules config files: ., /etc, /etc/scion-ftp")
	interval     = time.Duration(15 * time.Second) // Interval for Keep-Alive
)

func init() {
	flag.Parse()
}

type App struct {
	conn           *ftp.ServerConn
	out            io.Writer
	cmd            commandMap
	ctx            context.Context
	cancel         context.CancelFunc
	herculesBinary string
}

func (app *App) print(a interface{}) {
	fmt.Fprintln(app.out, a)
}

func (app *App) run() error {
	scanner := bufio.NewReader(os.Stdin)
	app.out = os.Stdout

	for {
		fmt.Printf("> ")
		input, err := scanner.ReadString('\n')
		if err != nil {
			return err
		}

		args := strings.Split(strings.TrimSpace(input), " ")
		if f, ok := app.cmd[args[0]]; ok {
			if app.conn == nil && args[0] != "help" && args[0] != "connect" {
				app.print("Need to make a connection first using \"connect\"")
				continue
			}
			f(args[1:])
		} else {
			fmt.Printf("Command %s does not exist\n", args[0])
		}
	}
}

func (app *App) help(args []string) {
	for cmd := range app.cmd {
		app.print(cmd)
	}
}

func (app *App) connect(args []string) {
	if len(args) != 1 {
		app.print("Must supply address to connect to")
		return
	}

	conn, err := ftp.Dial(args[0])
	if err != nil {
		app.print(err)
		return
	}

	app.conn = conn

	if app.conn.IsHerculesSupported() {
		app.print("This server supports Hercules up- and downloads, mode H for faster file transfers.")
	}

	ctx, cancel := context.WithCancel(app.ctx)
	app.cancel = cancel

	go app.keepalive(ctx, interval)
}

func (app *App) keepalive(ctx context.Context, interval time.Duration) {

	for {
		select {
		case <-time.After(interval):
			err := app.conn.NoOp()
			if err != nil {
				app.print(fmt.Sprintf("Failed to ping for keepalive: %s", err))
				return
			}
		case <-ctx.Done():
			return
		}
	}

}

func (app *App) login(args []string) {
	if len(args) != 2 {
		app.print("Must supply username and password")
		return
	}

	err := app.conn.Login(args[0], args[1])
	if err != nil {
		app.print(err)
	}
}

func (app *App) ls(args []string) {
	path := ""
	if len(args) == 1 {
		path = args[0]
	}

	entries, err := app.conn.List(path)

	if err != nil {
		app.print(err)
		return
	}

	for _, entry := range entries {
		app.print(entry.Name)
	}
}

func (app *App) cd(args []string) {
	if len(args) != 1 {
		app.print("Must supply one argument for directory change")
		return
	}

	err := app.conn.ChangeDir(args[0])
	if err != nil {
		app.print(err)
	}
}

func (app *App) mkdir(args []string) {
	if len(args) != 1 {
		app.print("Must supply one argument for directory name")
		return
	}

	err := app.conn.MakeDir(args[0])
	if err != nil {
		app.print(err)
	}
}

func (app *App) pwd(args []string) {
	cur, err := app.conn.CurrentDir()
	if err != nil {
		app.print(err)
	}
	app.print(cur)
}

func (app *App) mode(args []string) {
	if len(args) != 1 {
		app.print("Must supply one argument for mode, [S]tream, [E]xtended or [H]ercules")
		return
	}

	err := app.conn.Mode([]byte(args[0])[0])
	if err != nil {
		app.print(err)
	}
}

func (app *App) retr(args []string) {
	if len(args) < 2 || len(args) > 4 {
		app.print("Must supply one argument for source and one for destination, optionally one for offset and one for length")
		return
	}

	remotePath := args[0]
	localPath := args[1]
	offset := -1
	length := -1

	var resp *ftp.Response
	var err error

	if len(args) >= 3 {
		offset, err = strconv.Atoi(args[2])
		if err != nil {
			app.print("Failed to parse offset")
			return
		}
	}

	if len(args) == 4 {
		length, err = strconv.Atoi(args[3])
		if err != nil {
			app.print("Failed to parse length")
			return
		}
	}

	if app.conn.IsModeHercules() { // With Hercules, separation of data transmission and persistence is not possible
		if offset != -1 && length != -1 {
			app.print("ERET not supported with Hercules")
		} else if offset != -1 {
			err = app.conn.RetrHerculesFrom(app.herculesBinary, remotePath, localPath, int64(offset))
		} else {
			err = app.conn.RetrHercules(app.herculesBinary, remotePath, localPath)
		}
		if err != nil {
			app.print(err)
		}
	} else {
		if offset != -1 && length != -1 {
			resp, err = app.conn.Eret(remotePath, offset, length)
		} else if offset != -1 {
			resp, err = app.conn.RetrFrom(remotePath, uint64(offset))
		} else {
			resp, err = app.conn.Retr(remotePath)
		}

		if err != nil {
			app.print(err)
			return
		}
		defer resp.Close()

		f, err := os.Create(localPath)
		if err != nil {
			app.print(err)
			return
		}
		defer f.Close()

		n, err := io.Copy(f, resp)
		if err != nil {
			app.print(err)
		} else {
			app.print(fmt.Sprintf("Received %d bytes", n))
		}
	}
}

func (app *App) stor(args []string) {
	if len(args) != 2 {
		app.print("Must supply one argument for source and one for destination")
		return
	}

	var err error
	if app.conn.IsModeHercules() { // With Hercules, separation of data transmission and persistence is not possible
		err = app.conn.StorHercules(app.herculesBinary, args[0], args[1])
	} else {
		var f *os.File
		f, err = os.Open(args[0])
		if err != nil {
			app.print(err)
			return
		}

		err = app.conn.Stor(args[1], f)
	}
	if err != nil {
		app.print(err)
	}
}

func (app *App) quit(args []string) {
	if app.cancel != nil {
		app.cancel()
	}

	err := app.conn.Quit()
	if err != nil {
		app.print(err)
	} else {
		app.print("Goodbye")
	}

	os.Exit(0)
}
