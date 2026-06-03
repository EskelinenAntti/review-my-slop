package tui

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type App interface {
	Draw(io.Writer, int, int)
	Handle(Key, Terminal) (bool, error)
}

type Terminal interface {
	Enter() error
	Exit()
	Suspend(func() error) error
	Size() (int, int)
}

type Session struct {
	Input    io.Reader
	Output   io.Writer
	Terminal Terminal
}

func (s Session) Run(app App) error {
	input := s.Input
	if input == nil {
		input = os.Stdin
	}
	output := s.Output
	if output == nil {
		output = os.Stdout
	}
	term := s.Terminal
	if term == nil {
		term = &STTYTerminal{Input: os.Stdin, Output: os.Stdout}
	}
	if err := term.Enter(); err != nil {
		return err
	}
	defer term.Exit()

	for {
		rows, cols := term.Size()
		app.Draw(output, rows, cols)
		key, err := ReadKey(input)
		if err != nil {
			return err
		}
		done, err := app.Handle(key, term)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
}

type STTYTerminal struct {
	Input    *os.File
	Output   io.Writer
	settings string
}

func (t *STTYTerminal) Enter() error {
	if t.Input == nil {
		t.Input = os.Stdin
	}
	if t.Output == nil {
		t.Output = os.Stdout
	}
	cmd := exec.Command("stty", "-g")
	cmd.Stdin = t.Input
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	t.settings = strings.TrimSpace(string(out))
	raw := exec.Command("stty", "raw", "-echo")
	raw.Stdin = t.Input
	if err := raw.Run(); err != nil {
		return err
	}
	fmt.Fprint(t.Output, "\x1b[?1049h\x1b[?25l")
	return nil
}

func (t *STTYTerminal) Exit() {
	if t.Output == nil {
		t.Output = os.Stdout
	}
	fmt.Fprint(t.Output, "\x1b[?25h\x1b[?1049l")
	t.restore()
}

func (t *STTYTerminal) Suspend(fn func() error) error {
	if t.Output == nil {
		t.Output = os.Stdout
	}
	fmt.Fprint(t.Output, "\x1b[?25h\x1b[?1049l")
	t.restore()
	err := fn()
	raw := exec.Command("stty", "raw", "-echo")
	raw.Stdin = t.Input
	_ = raw.Run()
	fmt.Fprint(t.Output, "\x1b[?1049h\x1b[?25l")
	return err
}

func (t *STTYTerminal) Size() (int, int) {
	if t.Input == nil {
		t.Input = os.Stdin
	}
	cmd := exec.Command("stty", "size")
	cmd.Stdin = t.Input
	out, err := cmd.Output()
	if err != nil {
		return 24, 80
	}
	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		return 24, 80
	}
	rows, rowErr := strconv.Atoi(fields[0])
	cols, colErr := strconv.Atoi(fields[1])
	if rowErr != nil || rows < 1 {
		rows = 24
	}
	if colErr != nil || cols < 1 {
		cols = 80
	}
	return rows, cols
}

func (t *STTYTerminal) restore() {
	if t.settings == "" {
		return
	}
	cmd := exec.Command("stty", t.settings)
	cmd.Stdin = t.Input
	_ = cmd.Run()
}
