package main

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

func downloadAudio(youtubeUrl string, outputPath string) error {
	cmd := exec.Command("yt-dlp",
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"-o", outputPath,
		youtubeUrl)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error downloading audio: %w\nstderr: %s", err, stderr.String())
	}
	fmt.Println("Audio downloaded successfully.")
	return nil
}

func sendAudioToLemonfox(filePath string, apiKey string) (string, error) {
	fmt.Println("Starting transcription process...")

	// Read file
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file to form
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("error creating form file: %w", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(fileData)); err != nil {
		return "", fmt.Errorf("error copying file data: %w", err)
	}

	// Add other form fields
	writer.WriteField("language", "english")
	writer.WriteField("response_format", "vtt")
	writer.Close()

	// Create request
	req, err := http.NewRequest("POST", "https://api.lemonfox.ai/v1/audio/transcriptions", body)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	fmt.Println("Sending file to Lemonfox API...")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	fmt.Println("Request sent, waiting for transcription...")
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	// Add status code check and response logging
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	fmt.Printf("Status Code: %d\n", resp.StatusCode)
	fmt.Printf("Response Headers: %v\n", resp.Header)
	fmt.Printf("Transcription result: %s\n", string(respBody))
	return string(respBody), nil
}
