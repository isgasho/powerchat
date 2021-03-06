package main

import (
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/skratchdot/open-golang/open"
)

func myOpen(path1 string) error {
	if strings.HasPrefix(path1, "http://") || strings.HasPrefix(path1, "https://") {
		cmd := exec.Command("cmd", "/C", "start", path1)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return cmd.Start()
	}

	st, err := os.Stat(path1)
	if err != nil {
		return err
	}
	if st.IsDir() {
		err = open.StartWith(path1, "explorer.exe")
	} else {
		err = open.Start(path1)
	}
	return err
}
