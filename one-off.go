// Example of transcribing a single YouTube video
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		fmt.Printf("Error loading .env file: %v\n", err)
	}

	if err := runOneOff(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func runOneOff() error {
	youtubeUrl := "https://www.youtube.com/watch?v=wAzBl6xllzE"
	outputPath := filepath.Join(".", "downloaded_audio.mp3")
	
	apiKey := os.Getenv("LEMONFOX_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("LEMONFOX_API_KEY environment variable is not set")
	}

	if err := downloadAudio(youtubeUrl, outputPath); err != nil {
		return fmt.Errorf("download error: %v", err)
	}

	transcription, err := sendAudioToLemonfox(outputPath, apiKey)
	if err != nil {
		return fmt.Errorf("transcription error: %v", err)
	}
	
	fmt.Printf("Transcription: %s\n", transcription)
	return nil
} 