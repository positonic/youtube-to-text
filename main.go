package main

import (
	"fmt"
	"os"
	"path/filepath"
)


func main() {
	youtubeUrl := "https://www.youtube.com/watch?v=Y9QfOPxmxVI"
	outputPath := filepath.Join(".", "downloaded_audio.mp3")
	
	apiKey := os.Getenv("LEMONFOX_API_KEY")
	if apiKey == "" {
		fmt.Println("Error: LEMONFOX_API_KEY environment variable is not set")
		return
	}

	if err := downloadAudio(youtubeUrl, outputPath); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if err := sendAudioToLemonfox(outputPath, apiKey); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
} 