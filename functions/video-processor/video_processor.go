package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	fdk "github.com/fnproject/fdk-go"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/common/auth"
	"github.com/oracle/oci-go-sdk/v65/functions"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// CloudEvent represents the event from OCI Events Service
type CloudEvent struct {
	SpecVersion     string                 `json:"specversion"`
	Type            string                 `json:"type"`
	Source          string                 `json:"source"`
	ID              string                 `json:"id"`
	Time            string                 `json:"time"`
	DataContentType string                 `json:"datacontenttype"`
	Subject         string                 `json:"subject"`
	Data            map[string]interface{} `json:"data"`
}

// ObjectStorageEvent represents the Object Storage event data
type ObjectStorageEvent struct {
	EventType      string `json:"eventType"`
	CloudEventsVersion string `json:"cloudEventsVersion"`
	EventTypeVersion string `json:"eventTypeVersion"`
	Source         string `json:"source"`
	EventID        string `json:"eventId"`
	EventTime      string `json:"eventTime"`
	SchemaURL      string `json:"schemaURL"`
	ContentType    string `json:"contentType"`
	Data           struct {
		CompartmentID    string `json:"compartmentId"`
		CompartmentName  string `json:"compartmentName"`
		ResourceName     string `json:"resourceName"`
		ResourceID       string `json:"resourceId"`
		AvailabilityDomain string `json:"availabilityDomain"`
		FreeFormTags     map[string]string `json:"freeFormTags"`
		DefinedTags      map[string]map[string]interface{} `json:"definedTags"`
		Identity         struct {
			Type        string `json:"type"`
			PrincipalID string `json:"principalId"`
			PrincipalName string `json:"principalName"`
			UserAgent   string `json:"userAgent"`
		} `json:"identity"`
		Request struct {
			ID string `json:"id"`
		} `json:"request"`
		Response struct {
			Status         string `json:"status"`
			ResponseTime   string `json:"responseTime"`
			Headers        map[string][]string `json:"headers"`
		} `json:"response"`
		AdditionalDetails struct {
			BucketName   string `json:"bucketName"`
			BucketID     string `json:"bucketId"`
			Namespace    string `json:"namespace"`
			ObjectName   string `json:"objectName"`
			ObjectSize   int64  `json:"objectSize"`
			ETag         string `json:"eTag"`
			MD5          string `json:"md5"`
			VersionID    string `json:"versionId"`
			ArchivalState string `json:"archivalState"`
		} `json:"additionalDetails"`
	} `json:"data"`
}

// ProcessResponse represents the response from the video processor
type ProcessResponse struct {
	Status      string `json:"status"`
	Message     string `json:"message"`
	EventID     string `json:"event_id"`
	ObjectName  string `json:"object_name"`
	BucketName  string `json:"bucket_name"`
	ChunkJobID  string `json:"chunk_job_id,omitempty"`
	Error       string `json:"error,omitempty"`
	ProcessedAt string `json:"processed_at"`
}

var (
	namespace       = os.Getenv("OCI_NAMESPACE")
	region          = os.Getenv("OCI_REGION")
	mediaBucket     = os.Getenv("MEDIA_BUCKET")
	tempBucket      = os.Getenv("TEMP_BUCKET")
	chunkerEndpoint = os.Getenv("CHUNKER_ENDPOINT")
)

func main() {
	fdk.Handle(fdk.HandlerFunc(videoProcessorHandler))
}

func videoProcessorHandler(ctx context.Context, in io.Reader, out io.Writer) {
	start := time.Now()
	
	// Parse the cloud event
	var event CloudEvent
	if err := json.NewDecoder(in).Decode(&event); err != nil {
		writeErrorResponse(out, "Invalid cloud event format", err, "", "", start)
		return
	}

	log.Printf("Received event: %s from %s", event.Type, event.Source)

	// Handle Object Storage events
	if strings.Contains(event.Type, "objectstorage") && strings.Contains(event.Type, "object.create") {
		handleObjectUpload(ctx, event, out, start)
	} else {
		writeResponse(out, ProcessResponse{
			Status:      "ignored",
			Message:     "Event type not handled",
			EventID:     event.ID,
			ProcessedAt: time.Now().Format(time.RFC3339),
		})
	}
}

func handleObjectUpload(ctx context.Context, event CloudEvent, out io.Writer, start time.Time) {
	// Extract object details from event data
	data, ok := event.Data["data"].(map[string]interface{})
	if !ok {
		writeErrorResponse(out, "Invalid event data structure", fmt.Errorf("missing data field"), event.ID, "", start)
		return
	}

	additionalDetails, ok := data["additionalDetails"].(map[string]interface{})
	if !ok {
		writeErrorResponse(out, "Invalid event structure", fmt.Errorf("missing additionalDetails"), event.ID, "", start)
		return
	}

	bucketName := getString(additionalDetails, "bucketName")
	objectName := getString(additionalDetails, "objectName")
	objectSize := getInt64(additionalDetails, "objectSize")

	log.Printf("Processing upload: %s/%s (%d bytes)", bucketName, objectName, objectSize)

	// Only process videos from the media bucket
	if bucketName != mediaBucket {
		writeResponse(out, ProcessResponse{
			Status:      "ignored",
			Message:     fmt.Sprintf("Object not in media bucket (was in %s)", bucketName),
			EventID:     event.ID,
			ObjectName:  objectName,
			BucketName:  bucketName,
			ProcessedAt: time.Now().Format(time.RFC3339),
		})
		return
	}

	// Check if it's a video file
	if !isVideoFile(objectName) {
		writeResponse(out, ProcessResponse{
			Status:      "ignored", 
			Message:     "Not a video file",
			EventID:     event.ID,
			ObjectName:  objectName,
			BucketName:  bucketName,
			ProcessedAt: time.Now().Format(time.RFC3339),
		})
		return
	}

	// Step 1: Copy to temp bucket
	if err := copyToTempBucket(ctx, objectName); err != nil {
		writeErrorResponse(out, "Failed to copy to temp bucket", err, event.ID, objectName, start)
		return
	}

	// Step 2: Trigger video chunking
	chunkJobID, err := triggerVideoChunking(ctx, objectName)
	if err != nil {
		writeErrorResponse(out, "Failed to trigger video chunking", err, event.ID, objectName, start)
		return
	}

	// Success response
	writeResponse(out, ProcessResponse{
		Status:      "success",
		Message:     "Video processing initiated",
		EventID:     event.ID,
		ObjectName:  objectName,
		BucketName:  bucketName,
		ChunkJobID:  chunkJobID,
		ProcessedAt: time.Now().Format(time.RFC3339),
	})
}

func copyToTempBucket(ctx context.Context, objectName string) error {
	log.Printf("Copying %s to temp bucket %s", objectName, tempBucket)

	// Create OCI client with instance principal authentication
	provider, err := auth.InstancePrincipalConfigurationProvider()
	if err != nil {
		return fmt.Errorf("failed to create auth provider: %w", err)
	}

	client, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(provider)
	if err != nil {
		return fmt.Errorf("failed to create object storage client: %w", err)
	}

	// Copy object from media bucket to temp bucket
	copyReq := objectstorage.CopyObjectRequest{
		NamespaceName: &namespace,
		BucketName:    &tempBucket,
		CopyObjectDetails: objectstorage.CopyObjectDetails{
			SourceObjectName: &objectName,
			DestinationObjectName: &objectName,
			DestinationNamespace: &namespace,
			DestinationBucket: &tempBucket,
			SourceObjectIfMatchETag: nil, // Copy regardless of ETag
		},
	}

	resp, err := client.CopyObject(ctx, copyReq)
	if err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}

	log.Printf("Copy operation initiated with work request ID: %s", *resp.OpcWorkRequestId)
	return nil
}

func triggerVideoChunking(ctx context.Context, objectName string) (string, error) {
	log.Printf("Triggering video chunking for %s", objectName)

	// Construct the video URL in temp bucket
	videoURL := fmt.Sprintf("https://objectstorage.%s.oraclecloud.com/n/%s/b/%s/o/%s", 
		region, namespace, tempBucket, objectName)

	// Create chunking request
	chunkRequest := map[string]interface{}{
		"video_url":       videoURL,
		"target_duration": 60,
		"scene_threshold": 0.3,
		"max_deviation":   0.2,
		"source_bucket":   mediaBucket,
		"source_object":   objectName,
	}

	requestJSON, err := json.Marshal(chunkRequest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal chunk request: %w", err)
	}

	// Call the chunker function
	resp, err := http.Post(chunkerEndpoint, "application/json", strings.NewReader(string(requestJSON)))
	if err != nil {
		return "", fmt.Errorf("failed to call chunker endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chunker returned status %d", resp.StatusCode)
	}

	// Parse response to get job ID
	var chunkResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&chunkResponse); err != nil {
		return "", fmt.Errorf("failed to parse chunker response: %w", err)
	}

	// Generate a job ID for tracking
	jobID := fmt.Sprintf("chunk-%s-%d", 
		strings.ReplaceAll(objectName, ".", "-"), 
		time.Now().Unix())

	log.Printf("Chunking job %s initiated for %s", jobID, objectName)
	return jobID, nil
}

func isVideoFile(filename string) bool {
	videoExtensions := []string{
		".mp4", ".mov", ".avi", ".mkv", ".wmv", ".flv", 
		".webm", ".m4v", ".3gp", ".mxf", ".mts", ".m2ts",
	}

	filename = strings.ToLower(filename)
	for _, ext := range videoExtensions {
		if strings.HasSuffix(filename, ext) {
			return true
		}
	}
	return false
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

func writeResponse(out io.Writer, response ProcessResponse) {
	json.NewEncoder(out).Encode(response)
}

func writeErrorResponse(out io.Writer, message string, err error, eventID, objectName string, start time.Time) {
	log.Printf("Error: %s - %v", message, err)
	response := ProcessResponse{
		Status:      "error",
		Message:     message,
		EventID:     eventID,
		ObjectName:  objectName,
		Error:       err.Error(),
		ProcessedAt: time.Now().Format(time.RFC3339),
	}
	json.NewEncoder(out).Encode(response)
}