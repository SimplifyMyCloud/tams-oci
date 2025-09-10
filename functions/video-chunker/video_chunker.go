package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	fdk "github.com/fnproject/fdk-go"
)

// ChunkRequest represents the input for the video chunking function
type ChunkRequest struct {
	VideoURL       string  `json:"video_url"`
	TargetDuration float64 `json:"target_duration,omitempty"`
	SceneThreshold float64 `json:"scene_threshold,omitempty"`
	MaxDeviation   float64 `json:"max_deviation,omitempty"`
}

// ChunkResponse represents the output of the video chunking function
type ChunkResponse struct {
	Status       string      `json:"status"`
	Message      string      `json:"message"`
	ChunksCreated int        `json:"chunks_created"`
	Chunks       []ChunkInfo `json:"chunks"`
	ProcessingTime string    `json:"processing_time"`
	Error        string      `json:"error,omitempty"`
}

type ChunkInfo struct {
	ID        int     `json:"id"`
	StartTime float64 `json:"start_time"`
	EndTime   float64 `json:"end_time"`
	Duration  float64 `json:"duration"`
	URL       string  `json:"url"`
}

type SceneDetector struct {
	InputFile      string
	OutputDir      string
	TargetDuration float64
	SceneThreshold float64
	MaxDeviation   float64
}

type Scene struct {
	StartTime float64 `json:"start_time"`
	EndTime   float64 `json:"end_time"`
	Score     float64 `json:"score"`
}

type Chunk struct {
	ID        int
	StartTime float64
	EndTime   float64
	Duration  float64
	Scenes    []Scene
}

func main() {
	fdk.Handle(fdk.HandlerFunc(videoChunkerHandler))
}

func videoChunkerHandler(ctx context.Context, in io.Reader, out io.Writer) {
	start := time.Now()
	
	// Parse input
	var request ChunkRequest
	if err := json.NewDecoder(in).Decode(&request); err != nil {
		writeErrorResponse(out, "Invalid JSON input", err, start)
		return
	}

	// Set defaults from environment or request
	if request.TargetDuration == 0 {
		request.TargetDuration = getEnvFloat("TARGET_DURATION", 60.0)
	}
	if request.SceneThreshold == 0 {
		request.SceneThreshold = getEnvFloat("SCENE_THRESHOLD", 0.3)
	}
	if request.MaxDeviation == 0 {
		request.MaxDeviation = getEnvFloat("MAX_DEVIATION", 0.2)
	}

	// Create temporary working directory
	tempDir, err := os.MkdirTemp("/tmp", "video-chunks-*")
	if err != nil {
		writeErrorResponse(out, "Failed to create temp directory", err, start)
		return
	}
	defer os.RemoveAll(tempDir)

	// Download video file
	inputFile, err := downloadVideo(request.VideoURL, tempDir)
	if err != nil {
		writeErrorResponse(out, "Failed to download video", err, start)
		return
	}

	// Initialize scene detector
	detector := &SceneDetector{
		InputFile:      inputFile,
		OutputDir:      tempDir,
		TargetDuration: request.TargetDuration,
		SceneThreshold: request.SceneThreshold,
		MaxDeviation:   request.MaxDeviation,
	}

	// Process video
	chunks, err := detector.Process()
	if err != nil {
		writeErrorResponse(out, "Failed to process video", err, start)
		return
	}

	// Upload chunks and get URLs (placeholder - integrate with Object Storage)
	chunkInfos := make([]ChunkInfo, len(chunks))
	for i, chunk := range chunks {
		chunkInfos[i] = ChunkInfo{
			ID:        chunk.ID,
			StartTime: chunk.StartTime,
			EndTime:   chunk.EndTime,
			Duration:  chunk.Duration,
			URL:       fmt.Sprintf("https://objectstorage.region.oraclecloud.com/bucket/chunk_%03d.mp4", chunk.ID),
		}
	}

	// Return success response
	response := ChunkResponse{
		Status:         "success",
		Message:        "Video successfully chunked",
		ChunksCreated:  len(chunks),
		Chunks:         chunkInfos,
		ProcessingTime: time.Since(start).String(),
	}

	json.NewEncoder(out).Encode(response)
}

func downloadVideo(url, tempDir string) (string, error) {
	// Get filename from URL
	filename := filepath.Base(url)
	if filename == "." || filename == "/" {
		filename = "video.mp4"
	}
	
	outputPath := filepath.Join(tempDir, filename)
	
	// Download file
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	out, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer out.Close()
	
	_, err = io.Copy(out, resp.Body)
	return outputPath, err
}

func (sd *SceneDetector) Process() ([]Chunk, error) {
	// Detect scenes
	scenes, err := sd.DetectScenes()
	if err != nil {
		return nil, err
	}

	// If no scenes detected, create time-based scenes
	if len(scenes) == 0 {
		log.Println("No scenes detected, falling back to time-based splitting")
		scenes = sd.createTimeBasedScenes()
	}

	// Group scenes into chunks
	chunks := sd.GroupScenesIntoChunks(scenes)

	// Split video
	if err := sd.SplitVideo(chunks); err != nil {
		return nil, err
	}

	return chunks, nil
}

func (sd *SceneDetector) DetectScenes() ([]Scene, error) {
	log.Println("Detecting scenes in video...")

	// FFmpeg command to detect scenes
	cmd := exec.Command("ffmpeg",
		"-i", sd.InputFile,
		"-filter:v", fmt.Sprintf("select='gt(scene,%f)',showinfo", sd.SceneThreshold),
		"-f", "null",
		"-")

	output, err := cmd.CombinedOutput()
	if err != nil {
		// FFmpeg returns error even on success for null output
		if !strings.Contains(string(output), "Showinfo") {
			return nil, fmt.Errorf("failed to detect scenes: %v", err)
		}
	}

	// Parse FFmpeg output for scene timestamps
	scenes := sd.parseSceneOutput(string(output))
	log.Printf("Detected %d scene changes", len(scenes))

	return scenes, nil
}

func (sd *SceneDetector) parseSceneOutput(output string) []Scene {
	var scenes []Scene
	lines := strings.Split(output, "\n")

	lastTime := 0.0
	for _, line := range lines {
		if strings.Contains(line, "Showinfo") && strings.Contains(line, "pts_time") {
			parts := strings.Split(line, "pts_time:")
			if len(parts) > 1 {
				timePart := strings.Split(parts[1], " ")[0]
				if timestamp, err := strconv.ParseFloat(timePart, 64); err == nil {
					scenes = append(scenes, Scene{
						StartTime: lastTime,
						EndTime:   timestamp,
						Score:     0,
					})
					lastTime = timestamp
				}
			}
		}
	}

	return scenes
}

func (sd *SceneDetector) GroupScenesIntoChunks(scenes []Scene) []Chunk {
	log.Println("Grouping scenes into chunks...")

	var chunks []Chunk
	var currentChunk Chunk
	currentChunk.ID = 1
	currentChunk.StartTime = 0

	minDuration := sd.TargetDuration * (1 - sd.MaxDeviation)
	maxDuration := sd.TargetDuration * (1 + sd.MaxDeviation)

	for _, scene := range scenes {
		currentDuration := scene.EndTime - currentChunk.StartTime

		if currentDuration >= minDuration && len(currentChunk.Scenes) > 0 {
			currentChunk.EndTime = scene.StartTime
			currentChunk.Duration = currentChunk.EndTime - currentChunk.StartTime
			chunks = append(chunks, currentChunk)

			currentChunk = Chunk{
				ID:        len(chunks) + 1,
				StartTime: scene.StartTime,
				Scenes:    []Scene{scene},
			}
		} else if currentDuration > maxDuration && len(currentChunk.Scenes) > 0 {
			currentChunk.EndTime = scene.StartTime
			currentChunk.Duration = currentChunk.EndTime - currentChunk.StartTime
			chunks = append(chunks, currentChunk)

			currentChunk = Chunk{
				ID:        len(chunks) + 1,
				StartTime: scene.StartTime,
				Scenes:    []Scene{scene},
			}
		} else {
			currentChunk.Scenes = append(currentChunk.Scenes, scene)
		}
	}

	// Add final chunk
	if len(currentChunk.Scenes) > 0 {
		lastScene := currentChunk.Scenes[len(currentChunk.Scenes)-1]
		currentChunk.EndTime = lastScene.EndTime
		currentChunk.Duration = currentChunk.EndTime - currentChunk.StartTime
		chunks = append(chunks, currentChunk)
	}

	log.Printf("Created %d chunks from %d scenes", len(chunks), len(scenes))
	return chunks
}

func (sd *SceneDetector) SplitVideo(chunks []Chunk) error {
	log.Println("Splitting video into chunks...")

	baseName := strings.TrimSuffix(filepath.Base(sd.InputFile), filepath.Ext(sd.InputFile))

	for _, chunk := range chunks {
		outputFile := filepath.Join(sd.OutputDir,
			fmt.Sprintf("%s_chunk_%03d.mp4", baseName, chunk.ID))

		log.Printf("Creating chunk %d: %.2fs - %.2fs (%.2fs duration)",
			chunk.ID, chunk.StartTime, chunk.EndTime, chunk.Duration)

		cmd := exec.Command("ffmpeg",
			"-i", sd.InputFile,
			"-ss", fmt.Sprintf("%.3f", chunk.StartTime),
			"-t", fmt.Sprintf("%.3f", chunk.Duration),
			"-c", "copy",
			"-avoid_negative_ts", "make_zero",
			"-y",
			outputFile)

		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create chunk %d: %v\nOutput: %s",
				chunk.ID, err, string(output))
		}

		// TODO: Upload chunk to Object Storage here
		// uploadToObjectStorage(outputFile, chunk.ID)
	}

	log.Printf("Successfully created %d chunks", len(chunks))
	return nil
}

func (sd *SceneDetector) createTimeBasedScenes() []Scene {
	// Get video duration
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		sd.InputFile)

	output, err := cmd.Output()
	if err != nil {
		log.Printf("Failed to get video duration: %v", err)
		return nil
	}

	duration, _ := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)

	var scenes []Scene
	for t := 0.0; t < duration; t += 5.0 {
		endTime := t + 5.0
		if endTime > duration {
			endTime = duration
		}
		scenes = append(scenes, Scene{
			StartTime: t,
			EndTime:   endTime,
			Score:     0,
		})
	}

	return scenes
}

func writeErrorResponse(out io.Writer, message string, err error, start time.Time) {
	response := ChunkResponse{
		Status:         "error",
		Message:        message,
		Error:          err.Error(),
		ProcessingTime: time.Since(start).String(),
	}
	json.NewEncoder(out).Encode(response)
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultVal
}