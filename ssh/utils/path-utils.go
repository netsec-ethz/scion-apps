package utils

import (
	"os/user"
	"path"
	"strings"
)

func ParsePath(pth string) string {
	home := "/"

	usr, err := user.Current()
	if err == nil {
		home = usr.HomeDir
	}

	if pth == "~" {
		return path.Join(home, pth[1:])
	} else if strings.HasPrefix(pth, "~/") {
		return path.Join(home, pth[2:])
	} else {
		return pth
	}
}
