package internal

import (
	"flag"
	"fmt"
	"log"
	"syscall"

	"golang.org/x/term"
)

// ParseFlags parses command line flags
func ParseFlags() Config {
	var config Config
	flag.StringVar(&config.APIKey, "api-key", "", "Trakt API key")
	flag.StringVar(&config.TvFile, "tv", "", "Path to TV shows JSON file")
	flag.StringVar(&config.MovieFile, "movies", "", "Path to movies JSON file")
	flag.StringVar(&config.OutputFile, "output", "", "Output file path")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&config.NoProgress, "no-progress", false, "Disable progress bar")
	flag.BoolVar(&config.Force, "force", false, "Force update all entries, ignoring cache")
	flag.Parse()
	return config
}

// PromptForAPIKey prompts the user for API key
func PromptForAPIKey() string {
	fmt.Print("Enter Trakt API key: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatal("Failed to read API key:", err)
	}
	fmt.Println()
	return string(bytePassword)
}
