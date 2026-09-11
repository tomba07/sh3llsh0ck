package main

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

func selectOption(prompt string, options []string) int {
	selected := 0

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return 0
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	printMenu := func() {
		fmt.Print("\033[H\033[2J")
		fmt.Printf("%s\r\n\r\n", prompt)
		for i, opt := range options {
			if i == selected {
				fmt.Printf("  %s> %s%s\r\n", colorBrightGreen, opt, colorReset)
			} else {
				fmt.Printf("    %s\r\n", opt)
			}
		}
	}

	printMenu()

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			return selected
		}

		switch {
		case n == 1 && buf[0] == '\r':
			return selected

		case n >= 3 && buf[0] == 27 && buf[1] == '[':
			switch buf[2] {
			case 'A': // up
				if selected > 0 {
					selected--
				}
			case 'B': // down
				if selected < len(options)-1 {
					selected++
				}
			}
			printMenu()
		}
	}
}
