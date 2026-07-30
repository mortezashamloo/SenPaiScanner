package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/matinsenpai/senpaiscanner/internal/accessible"
	"github.com/matinsenpai/senpaiscanner/internal/ui"
	"github.com/matinsenpai/senpaiscanner/pkg/version"
)

func main() {
	if len(os.Args) > 1 {
		arg := strings.ToLower(os.Args[1])
		switch arg {
		case "--version", "-v", "version":
			fmt.Println("SenPai Scanner", version.String())
			return
		case "--accessible", "--simple", "-a", "accessible", "simple":
			accessible.Run(version.String())
			return
		case "--help", "-h", "help":
			printHelp()
			return
		}
	}

	model := ui.NewApp(version.Version)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Give the UI package a reference so background goroutines can send messages.
	ui.SetProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("SenPai Scanner", version.String())
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  senpaiscanner                 Launch the normal terminal UI")
	fmt.Println("  senpaiscanner --accessible     Launch plain-text mode for screen readers (NVDA/JAWS)")
	fmt.Println("  senpaiscanner --simple         Same as --accessible")
	fmt.Println("  senpaiscanner -a               Same as --accessible")
	fmt.Println("  senpaiscanner --version        Print version")
	fmt.Println("  senpaiscanner --help           Show this help")
	fmt.Println()
	fmt.Println("Accessible mode prints all progress and results as plain text,")
	fmt.Println("one line at a time, so screen readers can follow the scan easily.")
}
