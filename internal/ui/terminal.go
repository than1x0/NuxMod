package ui

import (
	"fmt"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	ColorBold   = "\033[1m"
)

// PrintBanner prints the application ASCII art banner.
func PrintBanner() {
	banner := `
  _   _             __  __           _ 
 | \ | |           |  \/  |         | |
 |  \| |_   ___  __| \  / | ___   __| |
 | . ` + "`" + ` | | | \ \/ /| |\/| |/ _ \ / _` + "`" + ` |
 | |\  | |_| |>  < | |  | | (_) | (_| |
 |_| \_|\__,_/_/\_\|_|  |_|\___/ \__,_|
  Linux System-Wide TUN Bypass & VPN Client
         Author: Sathira Laksahan
`
	fmt.Printf("%s%s%s\n", ColorCyan, banner, ColorReset)
}

func Info(format string, a ...any) {
	fmt.Printf("%s[*]%s %s\n", ColorCyan, ColorReset, fmt.Sprintf(format, a...))
}

func Success(format string, a ...any) {
	fmt.Printf("%s[+]%s %s\n", ColorGreen, ColorReset, fmt.Sprintf(format, a...))
}

func Warn(format string, a ...any) {
	fmt.Printf("%s[!]%s %s\n", ColorYellow, ColorReset, fmt.Sprintf(format, a...))
}

func Error(format string, a ...any) {
	fmt.Printf("%s[-]%s %s\n", ColorRed, ColorReset, fmt.Sprintf(format, a...))
}

func Header(text string) {
	fmt.Printf("\n%s%s=== %s ===%s\n", ColorBold, ColorBlue, text, ColorReset)
}
