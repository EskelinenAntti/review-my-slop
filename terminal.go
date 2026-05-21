package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func enterTerminal() (*terminalState, error) {
	cmd := exec.Command("stty", "-g")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	state := &terminalState{settings: strings.TrimSpace(string(out))}
	raw := exec.Command("stty", "raw", "-echo")
	raw.Stdin = os.Stdin
	if err := raw.Run(); err != nil {
		return nil, err
	}
	fmt.Print("\x1b[?1049h\x1b[?25l")
	return state, nil
}

func (t *terminalState) restore() {
	fmt.Print("\x1b[?25h\x1b[?1049l")
	if t.settings == "" {
		return
	}
	cmd := exec.Command("stty", t.settings)
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}

func withNormalTerminal(t *terminalState, fn func() error) error {
	fmt.Print("\x1b[?25h\x1b[?1049l")
	cmd := exec.Command("stty", t.settings)
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
	err := fn()
	raw := exec.Command("stty", "raw", "-echo")
	raw.Stdin = os.Stdin
	_ = raw.Run()
	fmt.Print("\x1b[?1049h\x1b[?25l")
	return err
}

func terminalSize() (int, int) {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return 24, 80
	}
	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		return 24, 80
	}
	rows, err := strconv.Atoi(fields[0])
	if err != nil {
		rows = 24
	}
	cols, err := strconv.Atoi(fields[1])
	if err != nil {
		cols = 80
	}
	return rows, cols
}
