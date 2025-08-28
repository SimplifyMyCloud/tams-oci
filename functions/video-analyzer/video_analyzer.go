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
	"github.com/oracle/oci-go-sdk/v65/aivision"
)

// AnalysisRequest represents the input for video analysis
type AnalysisRequest struct {
	ChunkURL      string `json:"chunk_url"`
	ChunkID       string `json:"chunk_id"`
	AssetID       string `json:"asset_id"`
	Timestamp     string `json:"timestamp"`
	AnalysisType  string `json:"analysis_type,omitempty"` // "objects", "labels", "text", "faces", "all"
}

// AnalysisResponse represents the analysis results
type AnalysisResponse struct {
	Status       string           `json:"status"`
	Message      string           `json:"message"`
	ChunkID      string           `json:"chunk_id"`
	AssetID      string           `json:"asset_id"`
	Analysis     VideoAnalysis    `json:"analysis"`
	ProcessingTime string         `json:"processing_time"`
	Error        string           `json:"error,omitempty"`
}

// VideoAnalysis holds the complete analysis results
type VideoAnalysis struct {
	Objects    []DetectedObject `json:"objects"`
	Labels     []DetectedLabel  `json:"labels"`
	Text       []DetectedText   `json:"text"`
	Faces      []DetectedFace   `json:"faces"`
	Summary    AnalysisSummary  `json:"summary"`
	Timeline   []TimelineEvent  `json:"timeline"`
}

type DetectedObject struct {
	Name       string      `json:"name"`
	Confidence float64     `json:"confidence"`
	BoundingBox BoundingBox `json:"bounding_box"`
	Timestamp  float64     `json:"timestamp"`
	FrameIndex int         `json:"frame_index"`
}

type DetectedLabel struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
	Timestamp  float64 `json:"timestamp"`
	Category   string  `json:"category"`
}

type DetectedText struct {
	Text       string      `json:"text"`
	Confidence float64     `json:"confidence"`
	BoundingBox BoundingBox `json:"bounding_box"`
	Timestamp  float64     `json:"timestamp"`
}

type DetectedFace struct {
	Confidence  float64     `json:"confidence"`
	BoundingBox BoundingBox `json:"bounding_box"`
	Timestamp   float64     `json:"timestamp"`
	Landmarks   []Landmark  `json:"landmarks,omitempty"`
}

type BoundingBox struct {
	Left   float64 `json:"left"`
	Top    float64 `json:"top"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type Landmark struct {
	Type string  `json:"type"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

type AnalysisSummary struct {
	ObjectCount    int      `json:"object_count"`
	LabelCount     int      `json:"label_count"`
	TextCount      int      `json:"text_count"`
	FaceCount      int      `json:"face_count"`
	TopObjects     []string `json:"top_objects"`
	TopLabels      []string `json:"top_labels"`
	ContentType    string   `json:"content_type"`    // "news", "sports", "documentary", etc.
	SceneComplexity string   `json:"scene_complexity"` // "low", "medium", "high"
}

type TimelineEvent struct {
	Timestamp   float64 `json:"timestamp"`
	EventType   string  `json:"event_type"`   // "object_appear", "text_detected", etc.
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
}

var (
	compartmentID = os.Getenv("OCI_COMPARTMENT_ID")
	tamsAPIEndpoint = os.Getenv("TAMS_API_ENDPOINT")
)

func main() {
	fdk.Handle(fdk.HandlerFunc(videoAnalyzerHandler))
}

func videoAnalyzerHandler(ctx context.Context, in io.Reader, out io.Writer) {
	start := time.Now()

	// Parse input request
	var request AnalysisRequest
	if err := json.NewDecoder(in).Decode(&request); err != nil {
		writeErrorResponse(out, "Invalid request format", err, "", "", start)
		return
	}

	log.Printf("Analyzing chunk: %s for asset: %s", request.ChunkID, request.AssetID)

	// Set default analysis type
	if request.AnalysisType == "" {
		request.AnalysisType = "all"
	}

	// Perform video analysis using OCI Vision
	analysis, err := analyzeVideoWithOCIVision(ctx, request)
	if err != nil {
		writeErrorResponse(out, "Failed to analyze video", err, request.ChunkID, request.AssetID, start)
		return
	}

	// Enhance with custom logic
	enhanceAnalysis(&analysis, request)

	// Send results to TAMS API
	if err := sendResultsToTAMS(ctx, request, analysis); err != nil {
		log.Printf("Warning: Failed to send results to TAMS API: %v", err)
		// Don't fail the function for API errors
	}

	// Return success response
	response := AnalysisResponse{
		Status:         "success",
		Message:        "Video analysis completed",
		ChunkID:        request.ChunkID,
		AssetID:        request.AssetID,
		Analysis:       analysis,
		ProcessingTime: time.Since(start).String(),
	}

	json.NewEncoder(out).Encode(response)
}

func analyzeVideoWithOCIVision(ctx context.Context, request AnalysisRequest) (VideoAnalysis, error) {
	log.Println("Starting OCI Vision analysis...")

	// Create OCI Vision client
	provider, err := auth.InstancePrincipalConfigurationProvider()
	if err != nil {
		return VideoAnalysis{}, fmt.Errorf("failed to create auth provider: %w", err)
	}

	client, err := aivision.NewAIServiceVisionClientWithConfigurationProvider(provider)
	if err != nil {
		return VideoAnalysis{}, fmt.Errorf("failed to create vision client: %w", err)
	}

	// Create analysis request for OCI Vision
	analysisReq := aivision.AnalyzeVideoRequest{
		AnalyzeVideoDetails: aivision.AnalyzeVideoDetails{
			Video: &aivision.ObjectStorageVideoDetails{
				Source:       aivision.VIDEO_SOURCE_OBJECT_STORAGE,
				NamespaceName: common.String(getNamespaceFromURL(request.ChunkURL)),
				BucketName:    common.String(getBucketFromURL(request.ChunkURL)),
				ObjectName:    common.String(getObjectFromURL(request.ChunkURL)),
			},
			Features: getAnalysisFeatures(request.AnalysisType),
			CompartmentId: &compartmentID,
		},
	}

	// Execute analysis
	resp, err := client.AnalyzeVideo(ctx, analysisReq)
	if err != nil {
		return VideoAnalysis{}, fmt.Errorf("failed to analyze video: %w", err)
	}

	// Convert OCI Vision response to our format
	analysis := convertOCIVisionResponse(resp.AnalyzeVideoResult)

	log.Printf("Analysis completed: %d objects, %d labels, %d text items, %d faces",
		len(analysis.Objects), len(analysis.Labels), len(analysis.Text), len(analysis.Faces))

	return analysis, nil
}

func getAnalysisFeatures(analysisType string) []aivision.VideoFeature {
	var features []aivision.VideoFeature

	switch analysisType {
	case "objects":
		features = append(features, aivision.VideoFeature{
			FeatureType: aivision.VIDEO_FEATURE_TYPE_OBJECT_DETECTION,
		})
	case "labels":
		features = append(features, aivision.VideoFeature{
			FeatureType: aivision.VIDEO_FEATURE_TYPE_LABEL_DETECTION,
		})
	case "text":
		features = append(features, aivision.VideoFeature{
			FeatureType: aivision.VIDEO_FEATURE_TYPE_TEXT_DETECTION,
		})
	case "faces":
		features = append(features, aivision.VideoFeature{
			FeatureType: aivision.VIDEO_FEATURE_TYPE_FACE_DETECTION,
		})
	default: // "all"
		features = []aivision.VideoFeature{
			{FeatureType: aivision.VIDEO_FEATURE_TYPE_OBJECT_DETECTION},
			{FeatureType: aivision.VIDEO_FEATURE_TYPE_LABEL_DETECTION},
			{FeatureType: aivision.VIDEO_FEATURE_TYPE_TEXT_DETECTION},
			{FeatureType: aivision.VIDEO_FEATURE_TYPE_FACE_DETECTION},
		}
	}

	return features
}

func convertOCIVisionResponse(result aivision.AnalyzeVideoResult) VideoAnalysis {
	analysis := VideoAnalysis{
		Objects:  []DetectedObject{},
		Labels:   []DetectedLabel{},
		Text:     []DetectedText{},
		Faces:    []DetectedFace{},
		Timeline: []TimelineEvent{},
	}

	// Convert objects
	if result.Objects != nil {
		for _, obj := range result.Objects {
			detectedObj := DetectedObject{
				Name:       *obj.Name,
				Confidence: *obj.Confidence,
				BoundingBox: BoundingBox{
					Left:   *obj.BoundingPolygon.NormalizedVertices[0].X,
					Top:    *obj.BoundingPolygon.NormalizedVertices[0].Y,
					Width:  *obj.BoundingPolygon.NormalizedVertices[2].X - *obj.BoundingPolygon.NormalizedVertices[0].X,
					Height: *obj.BoundingPolygon.NormalizedVertices[2].Y - *obj.BoundingPolygon.NormalizedVertices[0].Y,
				},
				Timestamp: *obj.TimeOffset,
			}
			analysis.Objects = append(analysis.Objects, detectedObj)

			// Add to timeline
			analysis.Timeline = append(analysis.Timeline, TimelineEvent{
				Timestamp:   *obj.TimeOffset,
				EventType:   "object_detected",
				Description: fmt.Sprintf("Detected %s", *obj.Name),
				Confidence:  *obj.Confidence,
			})
		}
	}

	// Convert labels
	if result.Labels != nil {
		for _, label := range result.Labels {
			detectedLabel := DetectedLabel{
				Name:       *label.Name,
				Confidence: *label.Confidence,
				Timestamp:  *label.TimeOffset,
				Category:   determineCategory(*label.Name),
			}
			analysis.Labels = append(analysis.Labels, detectedLabel)
		}
	}

	// Convert text
	if result.Texts != nil {
		for _, text := range result.Texts {
			detectedText := DetectedText{
				Text:       *text.Text,
				Confidence: *text.Confidence,
				Timestamp:  *text.TimeOffset,
				BoundingBox: BoundingBox{
					Left:   *text.BoundingPolygon.NormalizedVertices[0].X,
					Top:    *text.BoundingPolygon.NormalizedVertices[0].Y,
					Width:  *text.BoundingPolygon.NormalizedVertices[2].X - *text.BoundingPolygon.NormalizedVertices[0].X,
					Height: *text.BoundingPolygon.NormalizedVertices[2].Y - *text.BoundingPolygon.NormalizedVertices[0].Y,
				},
			}
			analysis.Text = append(analysis.Text, detectedText)
		}
	}

	// Convert faces
	if result.Faces != nil {
		for _, face := range result.Faces {
			detectedFace := DetectedFace{
				Confidence: *face.Confidence,
				Timestamp:  *face.TimeOffset,
				BoundingBox: BoundingBox{
					Left:   *face.BoundingPolygon.NormalizedVertices[0].X,
					Top:    *face.BoundingPolygon.NormalizedVertices[0].Y,
					Width:  *face.BoundingPolygon.NormalizedVertices[2].X - *face.BoundingPolygon.NormalizedVertices[0].X,
					Height: *face.BoundingPolygon.NormalizedVertices[2].Y - *face.BoundingPolygon.NormalizedVertices[0].Y,
				},
			}
			analysis.Faces = append(analysis.Faces, detectedFace)
		}
	}

	return analysis
}

func enhanceAnalysis(analysis *VideoAnalysis, request AnalysisRequest) {
	// Create summary
	analysis.Summary = AnalysisSummary{
		ObjectCount: len(analysis.Objects),
		LabelCount:  len(analysis.Labels),
		TextCount:   len(analysis.Text),
		FaceCount:   len(analysis.Faces),
	}

	// Determine top objects and labels
	objectCounts := make(map[string]int)
	labelCounts := make(map[string]int)

	for _, obj := range analysis.Objects {
		objectCounts[obj.Name]++
	}

	for _, label := range analysis.Labels {
		labelCounts[label.Name]++
	}

	// Get top 5 objects and labels
	analysis.Summary.TopObjects = getTopKeys(objectCounts, 5)
	analysis.Summary.TopLabels = getTopKeys(labelCounts, 5)

	// Determine content type based on detected objects/labels
	analysis.Summary.ContentType = determineContentType(analysis.Summary.TopLabels)

	// Determine scene complexity
	totalDetections := len(analysis.Objects) + len(analysis.Labels) + len(analysis.Text)
	if totalDetections < 10 {
		analysis.Summary.SceneComplexity = "low"
	} else if totalDetections < 50 {
		analysis.Summary.SceneComplexity = "medium"
	} else {
		analysis.Summary.SceneComplexity = "high"
	}
}

func sendResultsToTAMS(ctx context.Context, request AnalysisRequest, analysis VideoAnalysis) error {
	if tamsAPIEndpoint == "" {
		return nil // Skip if no API endpoint configured
	}

	// Create TAMS API payload
	payload := map[string]interface{}{
		"chunk_id":     request.ChunkID,
		"asset_id":     request.AssetID,
		"analysis":     analysis,
		"analyzed_at":  time.Now().Format(time.RFC3339),
		"analysis_type": "oci_vision",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Send to TAMS API
	resp, err := http.Post(
		fmt.Sprintf("%s/chunks/%s/analysis", tamsAPIEndpoint, request.ChunkID),
		"application/json",
		strings.NewReader(string(jsonData)),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("TAMS API returned status %d", resp.StatusCode)
	}

	log.Printf("Successfully sent analysis results to TAMS API")
	return nil
}

// Helper functions
func getNamespaceFromURL(url string) string {
	// Extract namespace from OCI Object Storage URL
	// Format: https://objectstorage.region.oraclecloud.com/n/namespace/b/bucket/o/object
	parts := strings.Split(url, "/")
	for i, part := range parts {
		if part == "n" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func getBucketFromURL(url string) string {
	parts := strings.Split(url, "/")
	for i, part := range parts {
		if part == "b" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func getObjectFromURL(url string) string {
	parts := strings.Split(url, "/")
	for i, part := range parts {
		if part == "o" && i+1 < len(parts) {
			return strings.Join(parts[i+1:], "/")
		}
	}
	return ""
}

func determineCategory(label string) string {
	// Basic categorization logic
	categories := map[string]string{
		"person": "people", "car": "vehicle", "building": "architecture",
		"text": "text", "logo": "branding", "face": "people",
	}
	
	for key, category := range categories {
		if strings.Contains(strings.ToLower(label), key) {
			return category
		}
	}
	return "general"
}

func determineContentType(labels []string) string {
	// Analyze labels to determine content type
	for _, label := range labels {
		label = strings.ToLower(label)
		if strings.Contains(label, "sport") || strings.Contains(label, "game") {
			return "sports"
		}
		if strings.Contains(label, "news") || strings.Contains(label, "studio") {
			return "news"
		}
		if strings.Contains(label, "music") || strings.Contains(label, "concert") {
			return "music"
		}
	}
	return "general"
}

func getTopKeys(counts map[string]int, limit int) []string {
	type kv struct {
		Key   string
		Value int
	}

	var pairs []kv
	for k, v := range counts {
		pairs = append(pairs, kv{k, v})
	}

	// Sort by count (descending)
	for i := 0; i < len(pairs)-1; i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[j].Value > pairs[i].Value {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}

	var result []string
	for i := 0; i < limit && i < len(pairs); i++ {
		result = append(result, pairs[i].Key)
	}

	return result
}

func writeErrorResponse(out io.Writer, message string, err error, chunkID, assetID string, start time.Time) {
	log.Printf("Error: %s - %v", message, err)
	response := AnalysisResponse{
		Status:         "error",
		Message:        message,
		ChunkID:        chunkID,
		AssetID:        assetID,
		Error:          err.Error(),
		ProcessingTime: time.Since(start).String(),
	}
	json.NewEncoder(out).Encode(response)
}