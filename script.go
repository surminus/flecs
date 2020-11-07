package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Script runs an arbitary command
type Script struct {
	Description string `yaml:"description"`
	Name        string `yaml:"name"`
	Path        string `yaml:"path"`
	Inline      string `yaml:"inline"`
}

func (s Script) Run() (cmd exec.Cmd, err error) {
	if s.Path != "" && s.Inline != "" {
		return cmd, fmt.Errorf("cannot define both path and inline")
	}

	if s.Path != "" {
		cmd = exec.Cmd{
			Path: "/bin/bash",
			Args: []string{"/bin/bash", s.Path},
		}
	}

	if s.Inline != "" {
		args := strings.Split(s.Inline, " ")
		if len(args) > 1 {
			cmd = *exec.Command(args[0], args[1:len(args)-1]...)
		} else {
			cmd = *exec.Command(s.Inline)
		}
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()

	return cmd, err
}
