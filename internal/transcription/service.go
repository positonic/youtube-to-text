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

func (s *Service) DownloadAudio(youtubeURL string, outputPath string) error {
	cmd := exec.Command("yt-dlp",
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"-o", outputPath,
		youtubeURL)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error downloading audio: %w", err)
	}
	return nil
}

func (s *Service) TranscribeAudio(filePath string) (string, error) {
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
	fmt.Printf("Connected to database: %s\n", s.dbURL)

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
		if err := s.DownloadAudio(video.VideoURL, outputPath); err != nil {
			s.transcriptionRepo.UpdateVideoStatus(video.ID, "failed")
			return fmt.Errorf("download error: %w", err)
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
