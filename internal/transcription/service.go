package transcription

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lib/pq"
	"jamesfarrell.me/youtube-to-text/internal/embeddings"
	"jamesfarrell.me/youtube-to-text/internal/storage/models"
	"jamesfarrell.me/youtube-to-text/internal/storage/postgres"
)

type Service struct {
	transcriptionRepo *postgres.TranscriptionRepository
	apiKey            string
	dbURL             string
}

func NewService(repo *postgres.TranscriptionRepository, apiKey string, dbURL string) *Service {
	return &Service{
		transcriptionRepo: repo,
		apiKey:           apiKey,
		dbURL:            dbURL,
	}
}

func (s *Service) DownloadAudio(youtubeURL string, outputPath string) (string, error) {
	// First, get the title using yt-dlp
	titleCmd := exec.Command("yt-dlp",
		"--get-title",
		youtubeURL)
	
	titleBytes, err := titleCmd.Output()
	if err != nil {
		return "", fmt.Errorf("error getting video title: %w", err)
	}
	title := strings.TrimSpace(string(titleBytes))

	// Then download the audio as before
	cmd := exec.Command("yt-dlp",
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"-o", outputPath,
		youtubeURL)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error downloading audio: %w", err)
	}
	
	// Check file size
	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		return "", fmt.Errorf("error checking file size: %w", err)
	}
	
	// 90MB is the max size for the audio file
	const maxSize = 90 * 1024 * 1024 // 90MB in bytes
	fmt.Printf("File size: %d bytes\n", fileInfo.Size())
	if fileInfo.Size() > maxSize {
		fmt.Println("Audio file too large, splitting...")
		// Get duration of the audio file
		durationCmd := exec.Command("ffprobe",
			"-v", "error",
			"-show_entries", "format=duration",
			"-of", "default=noprint_wrappers=1:nokey=1",
			outputPath)
		
		durationBytes, err := durationCmd.Output()
		if err != nil {
			return "", fmt.Errorf("error getting audio duration: %w", err)
		}
		
		var duration float64
		if _, err := fmt.Sscanf(string(durationBytes), "%f", &duration); err != nil {
			return "", fmt.Errorf("error parsing duration: %w", err)
		}

		// Calculate number of segments needed (aim for ~80MB per segment)
		numSegments := int(fileInfo.Size()/(80*1024*1024)) + 1
		segmentDuration := duration / float64(numSegments)
		fmt.Printf("Number of segments: %d\n", numSegments)
		fmt.Printf("Segment duration: %f seconds\n", segmentDuration)
		// Create a temporary directory for segments
		segmentDir := outputPath + "_segments"
		if err := os.MkdirAll(segmentDir, 0755); err != nil {
			return "", fmt.Errorf("error creating segments directory: %w", err)
		}
		
		// Split the file into segments
		splitCmd := exec.Command("ffmpeg",
			"-i", outputPath,
			"-f", "segment",
			"-segment_time", fmt.Sprintf("%f", segmentDuration),
			"-c", "copy",
			filepath.Join(segmentDir, "segment_%03d.mp3"))
		
		if err := splitCmd.Run(); err != nil {
			os.RemoveAll(segmentDir) // Clean up on error
			return "", fmt.Errorf("error splitting audio file: %w", err)
		}
		
		// Remove the original large file
		os.Remove(outputPath)
		
		// Return both the title and segment directory path
		return title, nil
	} else {
		fmt.Println("Audio file is within the size limit")
	}
	
	return title, nil
}

func (s *Service) TranscribeAudio(filePath string) (string, error) {
	segmentDir := filePath + "_segments"
	if _, err := os.Stat(segmentDir); err == nil {
		segments, err := filepath.Glob(filepath.Join(segmentDir, "segment_*.mp3"))
		if err != nil {
			return "", fmt.Errorf("error finding segments: %w", err)
		}
		
		if len(segments) > 1 {
			fmt.Printf("Processing %d segments from %s\n", len(segments), segmentDir)
		}
		
		var fullTranscription strings.Builder
		var totalDuration time.Duration
		
		fullTranscription.WriteString("WEBVTT\n\n")
		
		for i, segment := range segments {
			transcription, err := s.transcribeSegment(segment)
			if err != nil {
				return "", fmt.Errorf("error transcribing segment %s: %w", segment, err)
			}
			
			// Parse entries for both first and subsequent segments
			entries, err := ParseVTT(transcription)
			if err != nil {
				return "", fmt.Errorf("error parsing VTT segment %d: %w", i, err)
			}
			
			// Adjust timestamps and write entries
			for _, entry := range entries {
				if i > 0 {  // Only adjust timestamps for segments after first
					entry.Start += totalDuration
					entry.End += totalDuration
				}
				fmt.Fprintf(&fullTranscription, "%s --> %s\n%s\n\n",
					formatTimestamp(entry.Start),
					formatTimestamp(entry.End),
					entry.Text)
			}
			
			// Update total duration after each segment
			if len(entries) > 0 {
				totalDuration = entries[len(entries)-1].End
			}
		}
		
		os.RemoveAll(segmentDir)
		return fullTranscription.String(), nil
	}
	
	return s.transcribeSegment(filePath)
}

func (s *Service) transcribeSegment(filePath string) (string, error) {
	fmt.Println("Transcribing segment:", filePath)
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading file: %w", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", fmt.Errorf("error creating form file: %w", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(fileData)); err != nil {
		return "", fmt.Errorf("error copying file data: %w", err)
	}

	writer.WriteField("language", "english")
	writer.WriteField("response_format", "vtt")
	writer.Close()

	req, err := http.NewRequest("POST", "https://api.lemonfox.ai/v1/audio/transcriptions", body)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, respBody)
	}

	return string(respBody), nil
}

func (s *Service) ListenForNewVideos() error {
	listener := pq.NewListener(s.dbURL, 10*time.Second, time.Minute,
		func(ev pq.ListenerEventType, err error) {
			if err != nil {
				fmt.Printf("Listen error: %v\n", err)
			}
		})
	defer listener.Close()

	err := listener.Listen("new_video")
	if err != nil {
		return fmt.Errorf("listen error: %w", err)
	}

	fmt.Println("Listening for new videos...")

	for {
		select {
		case n := <-listener.Notify:
			if n == nil {
				fmt.Println("Received nil notification")
				continue
			}
			fmt.Printf("Received notification: %+v\n", n)
			var video models.Video
			if err := json.Unmarshal([]byte(n.Extra), &video); err != nil {
				fmt.Printf("Error unmarshaling notification: %v\n", err)
				continue
			}
			fmt.Printf("Received notification for video: %s\n", video.ID)
			// Process video...
			// Process the new video notification
			if err := s.processVideoNotification(n.Extra); err != nil {
				fmt.Printf("Error processing video: %v\n", err)
			} else {
				fmt.Println("Successfully processed video notification")
			}
		case <-time.After(time.Minute):
			fmt.Println("Ping check...")
			go func() {
				if err := listener.Ping(); err != nil {
					fmt.Printf("Ping error: %v\n", err)
				}
			}()
		}
	}
}

func (s *Service) processVideoNotification(notification string) error {
	var video models.Video
	if err := json.Unmarshal([]byte(notification), &video); err != nil {
		return fmt.Errorf("json parse error: %w", err)
	}

	fmt.Printf("Processing video ID: %s, URL: %s\n", video.ID, video.VideoURL)
	var transcription string

	// Check for existing transcription
	existingVideo, err := s.transcriptionRepo.GetByURL(video.VideoURL)
	if err == nil && existingVideo.Transcription != nil {
		fmt.Printf("Found existing transcription for video URL: %s\n", video.VideoURL)
		transcription = *existingVideo.Transcription
	} else {
		// If no existing transcription, proceed with download and transcribe
		fmt.Printf("No existing transcription found, processing video ID: %s, URL: %s\n", video.ID, video.VideoURL)
		
		if err := s.transcriptionRepo.UpdateVideoStatus(video.ID, "processing"); err != nil {
			return fmt.Errorf("failed to update status to processing: %w", err)
		}
		
		outputPath := fmt.Sprintf("./temp_%s.mp3", video.ID)
		defer os.Remove(outputPath)

		fmt.Printf("Downloading audio to: %s\n", outputPath)
		title, err := s.DownloadAudio(video.VideoURL, outputPath)
		if err != nil {
			s.transcriptionRepo.UpdateVideoStatus(video.ID, "failed")
			return fmt.Errorf("download error: %w", err)
		}
		fmt.Printf("Downloaded audio title: %s\n", title)
		// Save the video title
		if err := s.transcriptionRepo.UpdateVideoTitle(video.ID, title); err != nil {
			fmt.Printf("Warning: failed to save video title: %v\n", err)
			// Don't return error here as it's not critical to the main flow
		}
		fmt.Println("Audio download completed successfully")

		fmt.Println("Sending audio to Lemonfox for transcription...")
		
		transcription, err = s.TranscribeAudio(outputPath)
		if err != nil {
			s.transcriptionRepo.UpdateVideoStatus(video.ID, "failed")
			return fmt.Errorf("transcription error: %w", err)
		}
		fmt.Println("Transcription received", transcription)

		// Save full transcription first
		if err := s.transcriptionRepo.SaveFullTranscription(video.ID, transcription); err != nil {
			return fmt.Errorf("failed to save transcription: %w", err)
		}
	}

	if transcription == "" {
		return fmt.Errorf("no transcription found: neither existing nor newly generated transcription was successful")
	}

	fmt.Println("isSearchable:", video.IsSearchable)
	if video.IsSearchable {
		fmt.Println("isSearchable: Processing video ID:", video.ID)
		
		// 1. Parse VTT content
		vttEntries, err := ParseVTT(transcription)
		if err != nil {
			return fmt.Errorf("failed to parse VTT: %w", err)
		}
		fmt.Println("VTT entries:", vttEntries)
		// 2. Convert VTT to plain text for search
		plainText := ""
		for _, entry := range vttEntries {
			plainText += entry.Text + " "
		}
		plainText = strings.TrimSpace(plainText)
		fmt.Println("Plain text:", plainText)
		// 3. Create semantic search chunks from plain text
		chunks, err := s.chunkText(plainText, 30*time.Second, 5*time.Second)
		if err != nil {
			return fmt.Errorf("failed to create chunks: %w", err)
		}
		
		// 4. Save chunks with embeddings
		if err := s.transcriptionRepo.SaveChunks(video.ID, chunks); err != nil {
			return fmt.Errorf("failed to save chunks: %w", err)
		}
	}

	return s.transcriptionRepo.UpdateVideoStatus(video.ID, "completed")
}

func (s *Service) chunkText(plainText string, maxChunkDuration time.Duration, overlap time.Duration) ([]models.Chunk, error) {
    
	fmt.Println("Chunking text:", plainText)
	fmt.Println("Max chunk duration:", maxChunkDuration)
	fmt.Println("Overlap:", overlap)
	// Split text into sentences or paragraphs
    sentences := strings.Split(plainText, ". ")
    
    var chunks []models.Chunk
    var currentChunk strings.Builder
    chunkStartIndex := 0
    
    for i, sentence := range sentences {
        currentChunk.WriteString(sentence)
        currentChunk.WriteString(". ")
        
        // Create new chunk if we've accumulated enough text or it's the last sentence
        if i == len(sentences)-1 || currentChunk.Len() > 500 { // arbitrary length of ~500 chars per chunk
            text := strings.TrimSpace(currentChunk.String())
            
            // Generate embedding for the chunk
            embedding, err := embeddings.GetEmbedding(text, os.Getenv("OPENAI_API_KEY"))
            if err != nil {
                return nil, fmt.Errorf("failed to generate embedding: %w", err)
            }
            
            chunks = append(chunks, models.Chunk{
                Text:      text,
                StartTime: time.Duration(chunkStartIndex) * maxChunkDuration,
                EndTime:   time.Duration(i+1) * maxChunkDuration,
                Embedding: embedding,
            })
            
            // Start new chunk with overlap
            if i < len(sentences)-1 {
                currentChunk.Reset()
                // Add some overlap by including the last few sentences
                overlapStart := max(0, i-2) // overlap of ~2 sentences
                for j := overlapStart; j <= i; j++ {
                    currentChunk.WriteString(sentences[j])
                    currentChunk.WriteString(". ")
                }
                chunkStartIndex = overlapStart
            }
        }
    }
    
    return chunks, nil
}

func max(a, b int) int {
    if a > b {
        return a
    }
    return b
}

// Helper function to format Duration as VTT timestamp
func formatTimestamp(d time.Duration) string {
    h := d / time.Hour
    d -= h * time.Hour
    m := d / time.Minute
    d -= m * time.Minute
    s := d / time.Second
    d -= s * time.Second
    ms := d / time.Millisecond
    
    return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}
