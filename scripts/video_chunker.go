package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type SceneDetector struct {
	InputFile       string
	OutputDir       string
	TargetDuration  float64 // Target duration in seconds (60 for 1 minute)
	SceneThreshold  float64 // Scene detection threshold (0.0-1.0)
	MaxDeviation    float64 // Maximum deviation from target duration (e.g., 0.2 for 20%)
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

func NewSceneDetector(inputFile, outputDir string) *SceneDetector {
	return &SceneDetector{
		InputFile:      inputFile,
		OutputDir:      outputDir,
		TargetDuration: 60.0,  // 1 minute default
		SceneThreshold: 0.3,   // Scene detection sensitivity
		MaxDeviation:   0.2,    // Allow 20% deviation from target
	}
}

// DetectScenes uses FFmpeg to detect scene changes
func (sd *SceneDetector) DetectScenes() ([]Scene, error) {
	log.Println("Detecting scenes in video...")
	
	// Create temp file for scene data
	tempFile := filepath.Join(sd.OutputDir, "scenes_temp.txt")
	defer os.Remove(tempFile)
	
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

// parseSceneOutput extracts scene timestamps from FFmpeg output
func (sd *SceneDetector) parseSceneOutput(output string) []Scene {
	var scenes []Scene
	lines := strings.Split(output, "\n")
	
	lastTime := 0.0
	for _, line := range lines {
		if strings.Contains(line, "Showinfo") && strings.Contains(line, "pts_time") {
			// Extract timestamp from line like: pts_time:123.456
			parts := strings.Split(line, "pts_time:")
			if len(parts) > 1 {
				timePart := strings.Split(parts[1], " ")[0]
				if timestamp, err := strconv.ParseFloat(timePart, 64); err == nil {
					scenes = append(scenes, Scene{
						StartTime: lastTime,
						EndTime:   timestamp,
						Score:     0, // Could extract scene score if needed
					})
					lastTime = timestamp
				}
			}
		}
	}
	
	return scenes
}

// GroupScenesIntoChunks groups scenes into chunks near target duration
func (sd *SceneDetector) GroupScenesIntoChunks(scenes []Scene) []Chunk {
	log.Println("Grouping scenes into chunks...")
	
	var chunks []Chunk
	var currentChunk Chunk
	currentChunk.ID = 1
	currentChunk.StartTime = 0
	
	minDuration := sd.TargetDuration * (1 - sd.MaxDeviation)
	maxDuration := sd.TargetDuration * (1 + sd.MaxDeviation)
	
	for _, scene := range scenes {
		sceneDuration := scene.EndTime - scene.StartTime
		currentDuration := scene.EndTime - currentChunk.StartTime
		
		// Check if adding this scene would exceed our target
		if currentDuration >= minDuration && len(currentChunk.Scenes) > 0 {
			// Close current chunk at scene boundary
			currentChunk.EndTime = scene.StartTime
			currentChunk.Duration = currentChunk.EndTime - currentChunk.StartTime
			chunks = append(chunks, currentChunk)
			
			// Start new chunk
			currentChunk = Chunk{
				ID:        len(chunks) + 1,
				StartTime: scene.StartTime,
				Scenes:    []Scene{scene},
			}
		} else if currentDuration > maxDuration && len(currentChunk.Scenes) > 0 {
			// Force split if we exceed max duration
			currentChunk.EndTime = scene.StartTime
			currentChunk.Duration = currentChunk.EndTime - currentChunk.StartTime
			chunks = append(chunks, currentChunk)
			
			currentChunk = Chunk{
				ID:        len(chunks) + 1,
				StartTime: scene.StartTime,
				Scenes:    []Scene{scene},
			}
		} else {
			// Add scene to current chunk
			currentChunk.Scenes = append(currentChunk.Scenes, scene)
		}
	}
	
	// Add final chunk if it has content
	if len(currentChunk.Scenes) > 0 {
		lastScene := currentChunk.Scenes[len(currentChunk.Scenes)-1]
		currentChunk.EndTime = lastScene.EndTime
		currentChunk.Duration = currentChunk.EndTime - currentChunk.StartTime
		chunks = append(chunks, currentChunk)
	}
	
	log.Printf("Created %d chunks from %d scenes", len(chunks), len(scenes))
	return chunks
}

// SplitVideo splits the video into chunks using FFmpeg
func (sd *SceneDetector) SplitVideo(chunks []Chunk) error {
	log.Println("Splitting video into chunks...")
	
	// Create output directory
	if err := os.MkdirAll(sd.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}
	
	// Get input file base name
	baseName := strings.TrimSuffix(filepath.Base(sd.InputFile), filepath.Ext(sd.InputFile))
	
	for _, chunk := range chunks {
		outputFile := filepath.Join(sd.OutputDir, 
			fmt.Sprintf("%s_chunk_%03d.mp4", baseName, chunk.ID))
		
		log.Printf("Creating chunk %d: %.2fs - %.2fs (%.2fs duration)", 
			chunk.ID, chunk.StartTime, chunk.EndTime, chunk.Duration)
		
		// FFmpeg command to extract chunk
		cmd := exec.Command("ffmpeg",
			"-i", sd.InputFile,
			"-ss", fmt.Sprintf("%.3f", chunk.StartTime),
			"-t", fmt.Sprintf("%.3f", chunk.Duration),
			"-c", "copy",           // Copy codec for speed
			"-avoid_negative_ts", "make_zero",
			"-y",                   // Overwrite output
			outputFile)
		
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create chunk %d: %v\nOutput: %s", 
				chunk.ID, err, string(output))
		}
	}
	
	log.Printf("Successfully created %d chunks", len(chunks))
	return nil
}

// GenerateMetadata creates a JSON file with chunk information
func (sd *SceneDetector) GenerateMetadata(chunks []Chunk) error {
	metadata := map[string]interface{}{
		"source_file":     sd.InputFile,
		"target_duration": sd.TargetDuration,
		"total_chunks":    len(chunks),
		"created_at":      time.Now().Format(time.RFC3339),
		"chunks":          chunks,
	}
	
	metadataFile := filepath.Join(sd.OutputDir, "chunks_metadata.json")
	file, err := os.Create(metadataFile)
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %v", err)
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(metadata); err != nil {
		return fmt.Errorf("failed to write metadata: %v", err)
	}
	
	log.Printf("Metadata saved to %s", metadataFile)
	return nil
}

// Process runs the complete video chunking pipeline
func (sd *SceneDetector) Process() error {
	// Step 1: Detect scenes
	scenes, err := sd.DetectScenes()
	if err != nil {
		return err
	}
	
	// If no scenes detected, create time-based chunks
	if len(scenes) == 0 {
		log.Println("No scenes detected, falling back to time-based splitting")
		scenes = sd.createTimeBasedScenes()
	}
	
	// Step 2: Group scenes into chunks
	chunks := sd.GroupScenesIntoChunks(scenes)
	
	// Step 3: Split video
	if err := sd.SplitVideo(chunks); err != nil {
		return err
	}
	
	// Step 4: Generate metadata
	if err := sd.GenerateMetadata(chunks); err != nil {
		return err
	}
	
	return nil
}

// createTimeBasedScenes creates scenes based on fixed time intervals
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
	for t := 0.0; t < duration; t += 5.0 { // Create scene every 5 seconds
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

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: video_chunker <input_video> <output_directory> [target_duration_seconds]")
		os.Exit(1)
	}
	
	inputFile := os.Args[1]
	outputDir := os.Args[2]
	
	detector := NewSceneDetector(inputFile, outputDir)
	
	// Optional: Set custom target duration
	if len(os.Args) > 3 {
		if duration, err := strconv.ParseFloat(os.Args[3], 64); err == nil {
			detector.TargetDuration = duration
		}
	}
	
	log.Printf("Processing video: %s", inputFile)
	log.Printf("Target chunk duration: %.0f seconds", detector.TargetDuration)
	
	if err := detector.Process(); err != nil {
		log.Fatalf("Error processing video: %v", err)
	}
	
	log.Println("Video chunking complete!")
}