package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func openEditor(file string, line int) error {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}

	var cmd *exec.Cmd
	lineArg := fmt.Sprintf("+%d", line)
	if strings.ContainsAny(editor, " \t") {
		cmd = exec.Command("sh", "-c", fmt.Sprintf("%s %s %s", editor, shellQuote(lineArg), shellQuote(file)))
	} else {
		cmd = exec.Command(editor, lineArg, file)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func openEditorFile(file string) error {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}

	var cmd *exec.Cmd
	if strings.ContainsAny(editor, " \t") {
		cmd = exec.Command("sh", "-c", fmt.Sprintf("%s %s", editor, shellQuote(file)))
	} else {
		cmd = exec.Command(editor, file)
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
