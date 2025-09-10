package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	fdk "github.com/fnproject/fdk-go"
	_ "github.com/lib/pq"
)

type ChunkNotification struct {
	EventType   string `json:"eventType"`
	ObjectName  string `json:"objectName"`
	BucketName  string `json:"bucketName"`
	Size        int64  `json:"size"`
	EventTime   string `json:"eventTime"`
	ChunkID     string `json:"chunkId"`
	SourceAsset string `json:"sourceAsset"`
}

type NotificationResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	ChunkID   string `json:"chunk_id"`
	Updated   bool   `json:"updated"`
	Error     string `json:"error,omitempty"`
}

var (
	dbHost     = os.Getenv("DB_HOST")
	dbPassword = os.Getenv("DB_PASSWORD")
	dbName     = os.Getenv("DB_NAME")
	apiEndpoint = os.Getenv("API_ENDPOINT")
)

func main() {
	fdk.Handle(fdk.HandlerFunc(chunkNotifierHandler))
}

func chunkNotifierHandler(ctx context.Context, in io.Reader, out io.Writer) {
	var notification map[string]interface{}
	if err := json.NewDecoder(in).Decode(&notification); err != nil {
		writeErrorResponse(out, "Invalid notification format", err)
		return
	}

	// Parse ONS notification
	message, ok := notification["Message"].(string)
	if !ok {
		writeErrorResponse(out, "Invalid ONS message format", fmt.Errorf("missing Message field"))
		return
	}

	var event map[string]interface{}
	if err := json.Unmarshal([]byte(message), &event); err != nil {
		writeErrorResponse(out, "Failed to parse event message", err)
		return
	}

	// Extract event details
	data, ok := event["data"].(map[string]interface{})
	if !ok {
		writeErrorResponse(out, "Invalid event data", fmt.Errorf("missing data field"))
		return
	}

	additionalDetails, ok := data["additionalDetails"].(map[string]interface{})
	if !ok {
		writeErrorResponse(out, "Invalid additional details", fmt.Errorf("missing additionalDetails"))
		return
	}

	objectName := getString(additionalDetails, "objectName")
	bucketName := getString(additionalDetails, "bucketName") 
	objectSize := getInt64(additionalDetails, "objectSize")
	eventTime := getString(event, "eventTime")

	log.Printf("Processing chunk notification: %s/%s (%d bytes)", bucketName, objectName, objectSize)

	// Only process chunk files
	if !strings.Contains(objectName, "_chunk_") {
		writeResponse(out, NotificationResponse{
			Status:  "ignored",
			Message: "Not a chunk file",
		})
		return
	}

	// Extract chunk info from filename
	chunkInfo := parseChunkInfo(objectName)
	if chunkInfo == nil {
		writeErrorResponse(out, "Failed to parse chunk info", fmt.Errorf("invalid chunk filename: %s", objectName))
		return
	}

	// Update database
	updated, err := updateChunkStatus(chunkInfo.ChunkID, chunkInfo.SourceAsset, objectName, objectSize)
	if err != nil {
		writeErrorResponse(out, "Failed to update chunk status", err)
		return
	}

	// Notify TAMS API
	if err := notifyTAMSAPI(chunkInfo, objectName, objectSize); err != nil {
		log.Printf("Warning: Failed to notify TAMS API: %v", err)
		// Don't fail the function for API notification errors
	}

	writeResponse(out, NotificationResponse{
		Status:  "success",
		Message: "Chunk notification processed",
		ChunkID: chunkInfo.ChunkID,
		Updated: updated,
	})
}

type ChunkInfo struct {
	ChunkID     string
	SourceAsset string
	Sequence    int
}

func parseChunkInfo(objectName string) *ChunkInfo {
	// Expected format: video_name_chunk_001.mp4
	if !strings.Contains(objectName, "_chunk_") {
		return nil
	}

	parts := strings.Split(objectName, "_chunk_")
	if len(parts) != 2 {
		return nil
	}

	sourceAsset := parts[0]
	chunkPart := parts[1]
	
	// Remove extension
	chunkPart = strings.TrimSuffix(chunkPart, ".mp4")
	chunkPart = strings.TrimSuffix(chunkPart, ".mov")
	chunkPart = strings.TrimSuffix(chunkPart, ".avi")

	return &ChunkInfo{
		ChunkID:     fmt.Sprintf("%s_chunk_%s", sourceAsset, chunkPart),
		SourceAsset: sourceAsset,
		Sequence:    parseSequence(chunkPart),
	}
}

func parseSequence(chunkPart string) int {
	// Try to parse sequence number
	var seq int
	fmt.Sscanf(chunkPart, "%d", &seq)
	return seq
}

func updateChunkStatus(chunkID, sourceAsset, objectName string, size int64) (bool, error) {
	// Connect to database
	connStr := fmt.Sprintf("host=%s user=tamsapi password=%s dbname=%s sslmode=disable",
		dbHost, dbPassword, dbName)
	
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return false, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Update or insert chunk record
	query := `
		INSERT INTO chunks (chunk_id, asset_id, object_name, size, status, created_at)
		VALUES ($1, $2, $3, $4, 'completed', NOW())
		ON CONFLICT (chunk_id) 
		DO UPDATE SET 
			object_name = EXCLUDED.object_name,
			size = EXCLUDED.size,
			status = 'completed',
			updated_at = NOW()
		RETURNING chunk_id`

	var returnedChunkID string
	err = db.QueryRow(query, chunkID, sourceAsset, objectName, size).Scan(&returnedChunkID)
	if err != nil {
		return false, fmt.Errorf("failed to update chunk status: %w", err)
	}

	log.Printf("Updated chunk status: %s", returnedChunkID)
	return true, nil
}

func notifyTAMSAPI(chunkInfo *ChunkInfo, objectName string, size int64) error {
	if apiEndpoint == "" {
		return nil // Skip if no API endpoint configured
	}

	notification := map[string]interface{}{
		"event_type":    "chunk_completed",
		"chunk_id":      chunkInfo.ChunkID,
		"source_asset":  chunkInfo.SourceAsset,
		"object_name":   objectName,
		"size":          size,
		"sequence":      chunkInfo.Sequence,
		"timestamp":     time.Now().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	resp, err := http.Post(
		fmt.Sprintf("%s/chunks/notify", apiEndpoint),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return nil
}

func getString(data map[string]interface{}, key string) string {
	if val, ok := data[key].(string); ok {
		return val
	}
	return ""
}

func getInt64(data map[string]interface{}, key string) int64 {
	if val, ok := data[key].(float64); ok {
		return int64(val)
	}
	return 0
}

func writeResponse(out io.Writer, response NotificationResponse) {
	json.NewEncoder(out).Encode(response)
}

func writeErrorResponse(out io.Writer, message string, err error) {
	log.Printf("Error: %s - %v", message, err)
	response := NotificationResponse{
		Status:  "error",
		Message: message,
		Error:   err.Error(),
	}
	json.NewEncoder(out).Encode(response)
}