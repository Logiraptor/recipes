package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type config struct {
	baseURL string
	token   string
	jsonDir string
}

type recipeSummary struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type searchResponse struct {
	Items []recipeSummary `json:"items"`
}

type scrapePayload struct {
	IncludeTags bool   `json:"includeTags"`
	Data        string `json:"data"`
}

func main() {
	cfg := config{
		baseURL: envOr("MEALIE_BASE", "https://mealie.home.poyarzun.io"),
		token:   requireEnv("MEALIE_TOKEN"),
		jsonDir: "json",
	}
	if len(os.Args) > 1 {
		cfg.jsonDir = os.Args[1]
	}

	entries, err := os.ReadDir(cfg.jsonDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: directory %q: %v\n", cfg.jsonDir, err)
		os.Exit(1)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, filepath.Join(cfg.jsonDir, e.Name()))
		}
	}
	if len(files) == 0 {
		fmt.Printf("No .json files found in %s\n", cfg.jsonDir)
		return
	}

	var created, updated, failed int
	for _, f := range files {
		name, err := recipeName(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL %s — %v\n", f, err)
			failed++
			continue
		}

		slug, exists, err := findRecipe(cfg, name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL %s — search error: %v\n", f, err)
			failed++
			continue
		}

		if exists {
			if err := updateRecipe(cfg, f, slug); err != nil {
				fmt.Fprintf(os.Stderr, "FAIL %s — update error: %v\n", f, err)
				failed++
			} else {
				fmt.Printf("UPDATED  %s (%s)\n", name, f)
				updated++
			}
		} else {
			if err := createRecipe(cfg, f); err != nil {
				fmt.Fprintf(os.Stderr, "FAIL %s — create error: %v\n", f, err)
				failed++
			} else {
				fmt.Printf("CREATED  %s (%s)\n", name, f)
				created++
			}
		}
	}

	fmt.Printf("\nDone: %d created, %d updated, %d failed (out of %d files)\n",
		created, updated, failed, len(files))
	if failed > 0 {
		os.Exit(1)
	}
}

func recipeName(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var obj struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", err
	}
	if obj.Name == "" {
		return "", fmt.Errorf("missing name field")
	}
	return obj.Name, nil
}

func findRecipe(cfg config, name string) (slug string, exists bool, err error) {
	u, err := url.Parse(cfg.baseURL + "/api/recipes")
	if err != nil {
		return "", false, err
	}
	q := u.Query()
	q.Set("search", name)
	q.Set("perPage", "50")
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, err
	}

	var result searchResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", false, fmt.Errorf("parse search response: %w", err)
	}

	for _, item := range result.Items {
		if item.Name == name {
			return item.Slug, true, nil
		}
	}
	return "", false, nil
}

func createRecipe(cfg config, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var recipe map[string]any
	if err := json.Unmarshal(raw, &recipe); err != nil {
		return err
	}
	recipe["@context"] = "https://schema.org"
	recipe["@type"] = "Recipe"

	recipeJSON, err := json.Marshal(recipe)
	if err != nil {
		return err
	}

	payload := scrapePayload{
		IncludeTags: false,
		Data:        string(recipeJSON),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, cfg.baseURL+"/api/recipes/create/html-or-json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func updateRecipe(cfg config, path string, slug string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var recipe map[string]any
	if err := json.Unmarshal(raw, &recipe); err != nil {
		return err
	}

	patchURL := cfg.baseURL + "/api/recipes/" + url.PathEscape(slug)
	body, err := json.Marshal(recipe)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPatch, patchURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "Error: %s env var is required\n", key)
		os.Exit(1)
	}
	return v
}
