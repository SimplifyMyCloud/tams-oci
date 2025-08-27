package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type Config struct {
	Port        string
	APIBaseURL  string
	APIKey      string
}

type TAMSClient struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

type Asset struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Duration    int       `json:"duration"`
	Format      string    `json:"format"`
	Location    string    `json:"location"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Tags        []string  `json:"tags"`
	Chunks      []Chunk   `json:"chunks"`
}

type Chunk struct {
	ID        string    `json:"id"`
	AssetID   string    `json:"asset_id"`
	Sequence  int       `json:"sequence"`
	Duration  int       `json:"duration"`
	URL       string    `json:"url"`
	Bucket    string    `json:"bucket"`
	Size      int64     `json:"size"`
	Checksum  string    `json:"checksum"`
	CreatedAt time.Time `json:"created_at"`
}

type MigrationJob struct {
	ID          string    `json:"id"`
	AssetID     string    `json:"asset_id"`
	SourceType  string    `json:"source_type"`
	SourcePath  string    `json:"source_path"`
	Status      string    `json:"status"`
	Progress    float64   `json:"progress"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	Error       string    `json:"error"`
}

type SearchResult struct {
	Assets []Asset `json:"assets"`
	Total  int     `json:"total"`
	Page   int     `json:"page"`
	Limit  int     `json:"limit"`
}

type Dashboard struct {
	TotalAssets     int     `json:"total_assets"`
	TotalChunks     int     `json:"total_chunks"`
	StorageUsedGB   float64 `json:"storage_used_gb"`
	ActiveJobs      int     `json:"active_jobs"`
	CompletedJobs   int     `json:"completed_jobs"`
	FailedJobs      int     `json:"failed_jobs"`
	RecentAssets    []Asset `json:"recent_assets"`
}

type PageData struct {
	Title     string
	Active    string
	Data      interface{}
	Error     string
	Success   string
}

var (
	client *TAMSClient
	tmpl   *template.Template
)

func NewTAMSClient(baseURL, apiKey string) *TAMSClient {
	return &TAMSClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *TAMSClient) doRequest(method, endpoint string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, c.BaseURL+endpoint, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error: %s", string(respBody))
	}

	return respBody, nil
}

func (c *TAMSClient) GetDashboard() (*Dashboard, error) {
	data, err := c.doRequest("GET", "/api/v1/dashboard", nil)
	if err != nil {
		return nil, err
	}

	var dashboard Dashboard
	err = json.Unmarshal(data, &dashboard)
	return &dashboard, err
}

func (c *TAMSClient) GetAssets(page, limit int, search string) (*SearchResult, error) {
	endpoint := fmt.Sprintf("/api/v1/assets?page=%d&limit=%d", page, limit)
	if search != "" {
		endpoint += "&search=" + search
	}

	data, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var result SearchResult
	err = json.Unmarshal(data, &result)
	return &result, err
}

func (c *TAMSClient) GetAsset(id string) (*Asset, error) {
	data, err := c.doRequest("GET", "/api/v1/assets/"+id, nil)
	if err != nil {
		return nil, err
	}

	var asset Asset
	err = json.Unmarshal(data, &asset)
	return &asset, err
}

func (c *TAMSClient) CreateAsset(asset *Asset) (*Asset, error) {
	data, err := c.doRequest("POST", "/api/v1/assets", asset)
	if err != nil {
		return nil, err
	}

	var created Asset
	err = json.Unmarshal(data, &created)
	return &created, err
}

func (c *TAMSClient) GetMigrationJobs() ([]MigrationJob, error) {
	data, err := c.doRequest("GET", "/api/v1/migrations", nil)
	if err != nil {
		return nil, err
	}

	var jobs []MigrationJob
	err = json.Unmarshal(data, &jobs)
	return jobs, err
}

func (c *TAMSClient) StartMigration(job *MigrationJob) (*MigrationJob, error) {
	data, err := c.doRequest("POST", "/api/v1/migrations", job)
	if err != nil {
		return nil, err
	}

	var created MigrationJob
	err = json.Unmarshal(data, &created)
	return &created, err
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	dashboard, err := client.GetDashboard()
	
	data := PageData{
		Title:  "Dashboard",
		Active: "dashboard",
	}

	if err != nil {
		data.Error = "Failed to load dashboard: " + err.Error()
		dashboard = &Dashboard{}
	}
	data.Data = dashboard

	tmpl.ExecuteTemplate(w, "base", data)
}

func assetsHandler(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	
	result, err := client.GetAssets(1, 20, search)
	
	data := PageData{
		Title:  "Assets",
		Active: "assets",
	}

	if err != nil {
		data.Error = "Failed to load assets: " + err.Error()
		result = &SearchResult{Assets: []Asset{}}
	}
	data.Data = result

	tmpl.ExecuteTemplate(w, "base", data)
}

func assetDetailHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/asset/")
	
	asset, err := client.GetAsset(id)
	
	data := PageData{
		Title:  "Asset Detail",
		Active: "assets",
	}

	if err != nil {
		data.Error = "Failed to load asset: " + err.Error()
		http.Redirect(w, r, "/assets", http.StatusSeeOther)
		return
	}
	data.Data = asset

	tmpl.ExecuteTemplate(w, "base", data)
}

func migrationsHandler(w http.ResponseWriter, r *http.Request) {
	jobs, err := client.GetMigrationJobs()
	
	data := PageData{
		Title:  "Migrations",
		Active: "migrations",
	}

	if err != nil {
		data.Error = "Failed to load migrations: " + err.Error()
		jobs = []MigrationJob{}
	}
	data.Data = jobs

	tmpl.ExecuteTemplate(w, "base", data)
}

func newAssetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		r.ParseForm()
		
		asset := &Asset{
			Title:       r.FormValue("title"),
			Description: r.FormValue("description"),
			Format:      r.FormValue("format"),
			Status:      "pending",
			Tags:        strings.Split(r.FormValue("tags"), ","),
		}

		_, err := client.CreateAsset(asset)
		if err != nil {
			data := PageData{
				Title:  "New Asset",
				Active: "assets",
				Error:  "Failed to create asset: " + err.Error(),
			}
			tmpl.ExecuteTemplate(w, "base", data)
			return
		}

		http.Redirect(w, r, "/assets?success=Asset created successfully", http.StatusSeeOther)
		return
	}

	data := PageData{
		Title:  "New Asset",
		Active: "assets",
	}
	tmpl.ExecuteTemplate(w, "base", data)
}

func newMigrationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		r.ParseForm()
		
		job := &MigrationJob{
			AssetID:    r.FormValue("asset_id"),
			SourceType: r.FormValue("source_type"),
			SourcePath: r.FormValue("source_path"),
			Status:     "queued",
		}

		_, err := client.StartMigration(job)
		if err != nil {
			data := PageData{
				Title:  "New Migration",
				Active: "migrations",
				Error:  "Failed to start migration: " + err.Error(),
			}
			tmpl.ExecuteTemplate(w, "base", data)
			return
		}

		http.Redirect(w, r, "/migrations?success=Migration started successfully", http.StatusSeeOther)
		return
	}

	assets, _ := client.GetAssets(1, 100, "")
	
	data := PageData{
		Title:  "New Migration",
		Active: "migrations",
		Data:   assets,
	}
	tmpl.ExecuteTemplate(w, "base", data)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func main() {
	config := Config{
		Port:       getEnv("PORT", "8090"),
		APIBaseURL: getEnv("TAMS_API_URL", "http://localhost:8080"),
		APIKey:     getEnv("TAMS_API_KEY", ""),
	}

	client = NewTAMSClient(config.APIBaseURL, config.APIKey)

	var err error
	tmpl, err = template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatal("Error loading templates:", err)
	}

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/assets", assetsHandler)
	http.HandleFunc("/asset/", assetDetailHandler)
	http.HandleFunc("/assets/new", newAssetHandler)
	http.HandleFunc("/migrations", migrationsHandler)
	http.HandleFunc("/migrations/new", newMigrationHandler)
	http.HandleFunc("/health", healthHandler)

	log.Printf("TAMS Web UI starting on port %s", config.Port)
	log.Printf("Connecting to API at %s", config.APIBaseURL)
	
	log.Fatal(http.ListenAndServe(":"+config.Port, nil))
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}