package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/patrickoyarzun/recipes/internal/recipedata"
)

const trmnlMaxPayloadBytes = 2000

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
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	start := time.Now()
	slog.Info("starting trmnl-recipe")

	webhookURL := requireEnv("TRMNL_WEBHOOK_URL")

	store, err := recipedata.Open()
	if err != nil {
		slog.Error("failed to locate recipe data", "error", err)
		os.Exit(1)
	}
	slog.Info("using recipe data", "root", store.Root)

	now := time.Now()
	mealType := mealTypeAt(now)
	mealLabel := titleCase(mealType)
	slog.Info("resolved meal type", "time", now.Format(time.RFC3339), "hour", now.Hour(), "meal_type", mealType)

	today := now.Format(recipedata.DateLayout)
	week := recipedata.WeekStart(now)
	plan, err := store.MealPlan(week)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		slog.Info("no meal plan file for this week", "week", week)
		plan = &recipedata.MealPlan{WeekStart: week}
	case err != nil:
		slog.Error("failed to load this week's meal plan", "week", week, "error", err)
		os.Exit(1)
	}
	entries := entriesOn(plan, today)
	meals := make([]string, len(entries))
	for i, e := range entries {
		meals[i] = e.Meal
	}
	slog.Info("loaded today's entries", "week", week, "date", today, "count", len(entries), "meals", meals)

	entry := selectMeal(entries, mealType)
	if entry == nil {
		slog.Info("no meal found for type, pushing empty state", "meal_type", mealType)
		payload := webhookPayload{
			MergeVariables: mergeVariables{
				HasRecipe: false,
				MealType:  mealType,
				MealLabel: mealLabel,
				Message:   fmt.Sprintf("No %s planned for today.", mealType),
				UpdatedAt: now.Format(time.RFC3339),
			},
		}
		t1 := time.Now()
		if err := postWebhook(webhookURL, payload); err != nil {
			slog.Error("failed to push empty state to TRMNL", "error", err, "elapsed", time.Since(t1))
			os.Exit(1)
		}
		slog.Info("pushed empty state", "meal_type", mealType, "webhook_elapsed", time.Since(t1), "total_elapsed", time.Since(start))
		return
	}

	slug := entry.Recipe
	slog.Info("selected meal", "meal_type", mealType, "recipe_slug", slug)

	r, err := store.Recipe(slug)
	if err != nil {
		slog.Error("failed to load recipe", "slug", slug, "error", err)
		os.Exit(1)
	}
	slog.Info("loaded recipe", "slug", slug, "name", r.Name, "ingredients", len(r.RecipeIngredient), "instructions", len(r.RecipeInstructions))

	payload := webhookPayload{
		MergeVariables: mergeVariables{
			HasRecipe:    true,
			RecipeName:   r.Name,
			MealType:     mealType,
			MealLabel:    mealLabel,
			Ingredients:  r.RecipeIngredient,
			Instructions: r.RecipeInstructions,
			TotalTime:    r.TotalTime,
			PrepTime:     r.PrepTime,
			RecipeYield:  r.RecipeYield,
			UpdatedAt:    now.Format(time.RFC3339),
		},
	}

	origSize, _ := payloadSize(payload)
	fitted, err := fitPayload(payload, trmnlMaxPayloadBytes)
	if err != nil {
		slog.Error("failed to fit TRMNL payload", "original_bytes", origSize, "max_bytes", trmnlMaxPayloadBytes, "error", err)
		os.Exit(1)
	}
	fittedSize, _ := payloadSize(fitted)
	slog.Info("sized payload", "original_bytes", origSize, "fitted_bytes", fittedSize, "max_bytes", trmnlMaxPayloadBytes, "truncated", fitted.MergeVariables.Truncated)

	t2 := time.Now()
	if err := postWebhook(webhookURL, fitted); err != nil {
		slog.Error("failed to push recipe to TRMNL", "error", err, "elapsed", time.Since(t2))
		os.Exit(1)
	}
	slog.Info("pushed recipe to TRMNL", "recipe", fitted.MergeVariables.RecipeName, "meal_type", mealType, "webhook_elapsed", time.Since(t2), "total_elapsed", time.Since(start))
	if fitted.MergeVariables.Truncated {
		slog.Warn("payload was truncated", "note", fitted.MergeVariables.TruncatedNote)
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

// entriesOn returns the plan entries scheduled for the given date.
func entriesOn(plan *recipedata.MealPlan, date string) []recipedata.Entry {
	var out []recipedata.Entry
	for _, e := range plan.Entries {
		if e.Date == date {
			out = append(out, e)
		}
	}
	return out
}

func selectMeal(entries []recipedata.Entry, mealType string) *recipedata.Entry {
	for i := range entries {
		entry := &entries[i]
		if strings.EqualFold(entry.Meal, mealType) && entry.Recipe != "" {
			return entry
		}
	}
	return nil
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
		fitted.MergeVariables.TruncatedNote = "Recipe trimmed to fit TRMNL payload limits. See the full recipe at home."
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
		fitted.MergeVariables.Message = "Recipe too large for TRMNL. See the full instructions at home."
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
	slog.Debug("POST webhook", "url", endpoint, "body_bytes", len(body))

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
	slog.Debug("POST webhook response", "status", resp.StatusCode, "body_bytes", len(respBody))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(respBody, 200))
	}
	return nil
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
		slog.Error("required environment variable is not set", "key", key)
		os.Exit(1)
	}
	return v
}
