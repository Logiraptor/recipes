package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const trmnlMaxPayloadBytes = 2000

type planEntry struct {
	Date      string         `json:"date"`
	EntryType string         `json:"entryType"`
	Title     string         `json:"title"`
	RecipeID  *string        `json:"recipeId"`
	Recipe    *recipeSummary `json:"recipe"`
}

type mealplanPage struct {
	Items []planEntry `json:"items"`
}

type recipeSummary struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type recipe struct {
	Name               string        `json:"name"`
	RecipeYield        string        `json:"recipeYield"`
	TotalTime          string        `json:"totalTime"`
	PrepTime           string        `json:"prepTime"`
	RecipeIngredient   []ingredient  `json:"recipeIngredient"`
	RecipeInstructions []instruction `json:"recipeInstructions"`
}

type ingredient struct {
	Display      string          `json:"display"`
	Quantity     *float64        `json:"quantity"`
	Unit         *ingredientUnit `json:"unit"`
	Food         *ingredientFood `json:"food"`
	Note         *string         `json:"note"`
	OriginalText *string         `json:"originalText"`
}

type ingredientUnit struct {
	Name string `json:"name"`
}

type ingredientFood struct {
	Name string `json:"name"`
}

type instruction struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type webhookPayload struct {
	MergeVariables mergeVariables `json:"merge_variables"`
}

type mergeVariables struct {
	HasRecipe     bool     `json:"has_recipe"`
	RecipeName    string   `json:"recipe_name"`
	MealType      string   `json:"meal_type"`
	MealLabel     string   `json:"meal_label"`
	Ingredients   []string `json:"ingredients"`
	Instructions  []string `json:"instructions"`
	TotalTime     string   `json:"total_time,omitempty"`
	PrepTime      string   `json:"prep_time,omitempty"`
	RecipeYield   string   `json:"recipe_yield,omitempty"`
	Message       string   `json:"message,omitempty"`
	UpdatedAt     string   `json:"updated_at"`
	Truncated     bool     `json:"truncated"`
	TruncatedNote string   `json:"truncated_note,omitempty"`
}

func main() {
	baseURL := requireEnv("MEALIE_BASE")
	token := requireEnv("MEALIE_TOKEN")
	webhookURL := requireEnv("TRMNL_WEBHOOK_URL")

	now := time.Now()
	mealType := mealTypeAt(now)
	mealLabel := titleCase(mealType)

	entries, err := fetchTodayMealplans(baseURL, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching today's meal plan: %v\n", err)
		os.Exit(1)
	}

	entry := selectMeal(entries, mealType)
	if entry == nil {
		payload := webhookPayload{
			MergeVariables: mergeVariables{
				HasRecipe: false,
				MealType:  mealType,
				MealLabel: mealLabel,
				Message:   fmt.Sprintf("No %s planned for today.", mealType),
				UpdatedAt: now.Format(time.RFC3339),
			},
		}
		if err := postWebhook(webhookURL, payload); err != nil {
			fmt.Fprintf(os.Stderr, "Error pushing empty state to TRMNL: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Pushed empty state for %s.\n", mealType)
		return
	}

	slug := entry.Recipe.Slug
	r, err := fetchRecipe(baseURL, token, slug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching recipe %q: %v\n", slug, err)
		os.Exit(1)
	}

	payload := webhookPayload{
		MergeVariables: mergeVariables{
			HasRecipe:    true,
			RecipeName:   r.Name,
			MealType:     mealType,
			MealLabel:    mealLabel,
			Ingredients:  ingredientLines(r.RecipeIngredient),
			Instructions: instructionLines(r.RecipeInstructions),
			TotalTime:    r.TotalTime,
			PrepTime:     r.PrepTime,
			RecipeYield:  r.RecipeYield,
			UpdatedAt:    now.Format(time.RFC3339),
		},
	}

	fitted, err := fitPayload(payload, trmnlMaxPayloadBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error sizing TRMNL payload: %v\n", err)
		os.Exit(1)
	}

	if err := postWebhook(webhookURL, fitted); err != nil {
		fmt.Fprintf(os.Stderr, "Error pushing recipe to TRMNL: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Pushed %s recipe: %s\n", mealType, fitted.MergeVariables.RecipeName)
	if fitted.MergeVariables.Truncated {
		fmt.Println(fitted.MergeVariables.TruncatedNote)
	}
}

func mealTypeAt(now time.Time) string {
	hour := now.Hour()
	switch {
	case hour >= 5 && hour < 11:
		return "breakfast"
	case hour >= 11 && hour < 14:
		return "lunch"
	default:
		return "dinner"
	}
}

func selectMeal(entries []planEntry, mealType string) *planEntry {
	for i := range entries {
		entry := &entries[i]
		if strings.EqualFold(entry.EntryType, mealType) && entry.Recipe != nil && entry.Recipe.Slug != "" {
			return entry
		}
	}
	return nil
}

func fetchTodayMealplans(baseURL, token string) ([]planEntry, error) {
	endpoint, err := url.JoinPath(baseURL, "api", "households", "mealplans", "today")
	if err != nil {
		return nil, err
	}

	body, err := doGet(endpoint, token)
	if err != nil {
		return nil, err
	}

	var entries []planEntry
	if err := json.Unmarshal(body, &entries); err == nil {
		return entries, nil
	}

	var page mealplanPage
	if err := json.Unmarshal(body, &page); err == nil {
		return page.Items, nil
	}

	return nil, fmt.Errorf("parse today mealplans: %s", truncate(body, 200))
}

func fetchRecipe(baseURL, token, slug string) (*recipe, error) {
	endpoint, err := url.JoinPath(baseURL, "api", "recipes", slug)
	if err != nil {
		return nil, err
	}

	body, err := doGet(endpoint, token)
	if err != nil {
		return nil, err
	}

	var r recipe
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("parse recipe: %w", err)
	}
	return &r, nil
}

func ingredientLines(ingredients []ingredient) []string {
	lines := make([]string, 0, len(ingredients))
	for _, ing := range ingredients {
		line := ingredientLine(ing)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func instructionLines(instructions []instruction) []string {
	lines := make([]string, 0, len(instructions))
	for _, step := range instructions {
		text := strings.TrimSpace(step.Text)
		title := strings.TrimSpace(step.Title)

		switch {
		case title != "" && text != "":
			lines = append(lines, title+": "+text)
		case text != "":
			lines = append(lines, text)
		case title != "":
			lines = append(lines, title)
		}
	}
	return lines
}

func ingredientLine(ing ingredient) string {
	if ing.Display != "" {
		return ing.Display
	}
	if ing.OriginalText != nil && *ing.OriginalText != "" {
		return *ing.OriginalText
	}

	var parts []string
	if ing.Quantity != nil && *ing.Quantity > 0 {
		q := *ing.Quantity
		if q == float64(int(q)) {
			parts = append(parts, fmt.Sprintf("%d", int(q)))
		} else {
			parts = append(parts, fmt.Sprintf("%.2g", q))
		}
	}
	if ing.Unit != nil && ing.Unit.Name != "" {
		parts = append(parts, ing.Unit.Name)
	}
	if ing.Food != nil && ing.Food.Name != "" {
		parts = append(parts, ing.Food.Name)
	}
	if ing.Note != nil && *ing.Note != "" {
		parts = append(parts, "("+*ing.Note+")")
	}
	return strings.Join(parts, " ")
}

func fitPayload(payload webhookPayload, maxBytes int) (webhookPayload, error) {
	fitted := payload
	fitted.MergeVariables.Ingredients = append([]string(nil), payload.MergeVariables.Ingredients...)
	fitted.MergeVariables.Instructions = append([]string(nil), payload.MergeVariables.Instructions...)

	size, err := payloadSize(fitted)
	if err != nil {
		return webhookPayload{}, err
	}
	if size <= maxBytes {
		return fitted, nil
	}

	truncated := false
	for len(fitted.MergeVariables.Instructions) > 0 {
		fitted.MergeVariables.Instructions = fitted.MergeVariables.Instructions[:len(fitted.MergeVariables.Instructions)-1]
		size, err = payloadSize(fitted)
		if err != nil {
			return webhookPayload{}, err
		}
		truncated = true
		if size <= maxBytes {
			break
		}
	}

	for size > maxBytes && len(fitted.MergeVariables.Ingredients) > 0 {
		fitted.MergeVariables.Ingredients = fitted.MergeVariables.Ingredients[:len(fitted.MergeVariables.Ingredients)-1]
		size, err = payloadSize(fitted)
		if err != nil {
			return webhookPayload{}, err
		}
		truncated = true
	}

	if truncated {
		fitted.MergeVariables.Truncated = true
		fitted.MergeVariables.TruncatedNote = "Recipe trimmed to fit TRMNL payload limits. Open Mealie for the full recipe."
	}

	for size > maxBytes && fitted.MergeVariables.TruncatedNote != "" {
		fitted.MergeVariables.TruncatedNote = ""
		size, err = payloadSize(fitted)
		if err != nil {
			return webhookPayload{}, err
		}
	}

	for size > maxBytes && fitted.MergeVariables.TotalTime != "" {
		fitted.MergeVariables.TotalTime = ""
		size, err = payloadSize(fitted)
		if err != nil {
			return webhookPayload{}, err
		}
	}

	for size > maxBytes && fitted.MergeVariables.PrepTime != "" {
		fitted.MergeVariables.PrepTime = ""
		size, err = payloadSize(fitted)
		if err != nil {
			return webhookPayload{}, err
		}
	}

	for size > maxBytes && fitted.MergeVariables.RecipeYield != "" {
		fitted.MergeVariables.RecipeYield = ""
		size, err = payloadSize(fitted)
		if err != nil {
			return webhookPayload{}, err
		}
	}

	if size > maxBytes {
		fitted.MergeVariables.Instructions = nil
		fitted.MergeVariables.Message = "Recipe too large for TRMNL. Open Mealie for the full instructions."
		size, err = payloadSize(fitted)
		if err != nil {
			return webhookPayload{}, err
		}
	}

	for size > maxBytes && len(fitted.MergeVariables.Ingredients) > 0 {
		fitted.MergeVariables.Ingredients = fitted.MergeVariables.Ingredients[:len(fitted.MergeVariables.Ingredients)-1]
		size, err = payloadSize(fitted)
		if err != nil {
			return webhookPayload{}, err
		}
	}

	if size > maxBytes {
		return webhookPayload{}, fmt.Errorf("payload still %d bytes after trimming to %d bytes", size, maxBytes)
	}

	return fitted, nil
}

func payloadSize(payload webhookPayload) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	return len(body), nil
}

func postWebhook(endpoint string, payload webhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(respBody, 200))
	}
	return nil
}

func doGet(endpoint, token string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(body, 200))
	}
	return body, nil
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "Error: %s env var is required\n", key)
		os.Exit(1)
	}
	return v
}
