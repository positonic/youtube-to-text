package transcription

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	writer.WriteField("response_format", "json")
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
				continue
			}
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
			go listener.Ping()
		}
	}
}

func (s *Service) processVideoNotification(jsonData string) error {
	var video models.Video
	if err := json.Unmarshal([]byte(jsonData), &video); err != nil {
		return fmt.Errorf("json parse error: %w", err)
	}

	fmt.Printf("Processing video ID: %s, URL: %s\n", video.ID, video.VideoURL)
	
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
    
	// transcription := `{"text":"In today's video will be exposing, exposing myself, no, exposing my YouTube analytics and just going over sort of general interest indicators for Bitcoin. Again, things are, you know, more or less slow here. We'll do a slight amount of setup and tentacle analysis at the end of this video, but I just want to be showing this sort of data that is not available to everyone. In fact, it's available to very few people, which is of course my YouTube analytics. So I'm exposing myself right now. "}`
	transcription, err := s.TranscribeAudio(outputPath)
	if err != nil {
		s.transcriptionRepo.UpdateVideoStatus(video.ID, "failed")
		return fmt.Errorf("transcription error: %w", err)
	}
	fmt.Println("Transcription received", transcription)
	if err := s.transcriptionRepo.SaveFullTranscription(video.ID, transcription); err != nil {
		return fmt.Errorf("failed to save transcription: %w", err)
	}
	fmt.Println("Updating final status to completed...")
    if video.IsSearchable {
		fmt.Println("isSearchable: Saving chunks for video ID:", video.ID)
		chunks, err := s.chunkText(transcription, 500, 50)
		if err != nil {
			updateErr := s.transcriptionRepo.UpdateVideoStatus(video.ID, "chunk_processing_failed")
			if updateErr != nil {
				log.Printf("Failed to update video status: %v", updateErr)
			}
			return fmt.Errorf("failed to create chunks: %w", err)
		}
		fmt.Printf("Number of chunks: %d\n", len(chunks))
		if err := s.transcriptionRepo.SaveChunks(video.ID, chunks); err != nil {
			updateErr := s.transcriptionRepo.UpdateVideoStatus(video.ID, "chunk_processing_failed")
			if updateErr != nil {
				log.Printf("Failed to update video status: %v", updateErr)
			}
			return fmt.Errorf("failed to save chunks: %w", err)
		}
		
		fmt.Println("Chunks saved successfully for video ID:", video.ID)
		if err := s.transcriptionRepo.UpdateVideoStatus(video.ID, "completed"); err != nil {
			return fmt.Errorf("failed to update video status: %w", err)
		}
	}
	return s.transcriptionRepo.UpdateVideoStatus(video.ID, "completed")
}

func (s *Service) chunkText(text string, chunkSize, overlap int) ([]models.Chunk, error) {
    var chunks []models.Chunk
    sentences := strings.Split(text, ".")
    currentChunk := ""
    startPos := 0
    
    for _, sentence := range sentences {
        sentence = strings.TrimSpace(sentence) + "."
        if len(currentChunk)+len(sentence) > chunkSize && len(currentChunk) > 0 {
            // Generate embedding for the chunk
            embedding, err := embeddings.GetEmbedding(currentChunk, os.Getenv("OPENAI_API_KEY"))
            if err != nil {
                return nil, fmt.Errorf("failed to generate embedding: %w", err)
            }
            
            chunks = append(chunks, models.Chunk{
                Text:          currentChunk,
                StartPosition: startPos,
                EndPosition:   startPos + len(currentChunk),
                Embedding:     embedding,
            })
            currentChunk = currentChunk[len(currentChunk)-overlap:] + sentence
            startPos = startPos + len(currentChunk) - overlap
        } else {
            currentChunk += sentence
        }
    }
    
    if len(currentChunk) > 0 {
        // Generate embedding for the final chunk
        embedding, err := embeddings.GetEmbedding(currentChunk, os.Getenv("OPENAI_API_KEY"))
        if err != nil {
            return nil, fmt.Errorf("failed to generate embedding: %w", err)
        }
        
        chunks = append(chunks, models.Chunk{
            Text:          currentChunk,
            StartPosition: startPos,
            EndPosition:   startPos + len(currentChunk),
            Embedding:     embedding,
        })
    }
    
    return chunks, nil
}
