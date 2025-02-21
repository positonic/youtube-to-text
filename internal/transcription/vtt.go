package transcription

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"jamesfarrell.me/youtube-to-text/internal/storage/models"
)

// ParseVTT parses WebVTT content into SRT entries
func ParseVTT(content string) ([]models.SRTEntry, error) {
	// Trim any quotes from the content
	content = strings.Trim(content, "\"")

	// Convert literal \n to actual newlines if needed
	if strings.Contains(content, "\\n") {
		content = strings.ReplaceAll(content, "\\n", "\n")
	}

	// Now validate the header with actual newlines
	if !strings.HasPrefix(content, "WEBVTT\n\n") {
		return nil, fmt.Errorf("invalid VTT format: missing WEBVTT header")
	}
	content = strings.TrimPrefix(content, "WEBVTT\n\n")

	// Split into entries by double newline
	entries := []models.SRTEntry{}
	blocks := strings.Split(content, "\n\n")
	
	for i, block := range blocks {
		// Split each block into lines
		lines := strings.Split(block, "\n")
		if len(lines) < 2 {
			continue
		}

		// First line should be timestamp
		timestamps := strings.Split(lines[0], " --> ")
		if len(timestamps) != 2 {
			continue
		}

		start, err := parseVTTTimestamp(timestamps[0])
		if err != nil {
			return nil, fmt.Errorf("invalid start timestamp: %w", err)
		}

		end, err := parseVTTTimestamp(timestamps[1])
		if err != nil {
			return nil, fmt.Errorf("invalid end timestamp: %w", err)
		}

		// Remaining lines are the text
		text := strings.Join(lines[1:], " ")

		entries = append(entries, models.SRTEntry{
			Number: i + 1,
			Start:  start,
			End:    end,
			Text:   text,
		})
	}

	return entries, nil
}

func parseVTTTimestamp(timestamp string) (time.Duration, error) {
	// Validate format (HH:MM:SS.mmm)
	if !strings.Contains(timestamp, ".") {
		return 0, fmt.Errorf("invalid timestamp format: missing milliseconds")
	}

	// Validate timestamp has exactly 2 digits for hours
	parts := strings.Split(timestamp, ":")
	if len(parts) != 3 || len(parts[0]) != 2 {
		return 0, fmt.Errorf("invalid timestamp format: expected HH:MM:SS.mmm")
	}

	// Parse hours
	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid hours: %w", err)
	}

	// Parse minutes
	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid minutes: %w", err)
	}

	// Split seconds and milliseconds
	secondParts := strings.Split(parts[2], ".")
	if len(secondParts) != 2 {
		return 0, fmt.Errorf("invalid seconds format: missing milliseconds")
	}

	// Parse seconds
	seconds, err := strconv.Atoi(secondParts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid seconds: %w", err)
	}

	// Parse milliseconds
	milliseconds, err := strconv.Atoi(secondParts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid milliseconds: %w", err)
	}

	duration := time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		time.Duration(milliseconds)*time.Millisecond

	return duration, nil
} 