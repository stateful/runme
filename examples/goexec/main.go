package main

import (
	"log"
	"os"
	"os/exec"
)

func main() {
	cmd := exec.Cmd{
		Path:   "/usr/local/bin/bash",
		Args:   []string{"-l", "-c", "python"},
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("failed to start: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		log.Fatalf("failed to wait: %v", err)
	}
}
