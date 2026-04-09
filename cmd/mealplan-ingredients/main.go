package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

type mealplanPage struct {
	Items []planEntry `json:"items"`
}

type planEntry struct {
	Date      string         `json:"date"`
	EntryType string         `json:"entryType"`
	Title     string         `json:"title"`
	RecipeID  *string        `json:"recipeId"`
	Recipe    *recipeSummary `json:"recipe"`
}

type recipeSummary struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type recipe struct {
	Name             string       `json:"name"`
	RecipeIngredient []ingredient `json:"recipeIngredient"`
}

type ingredient struct {
	Display      string `json:"display"`
	Quantity     *float64
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

func main() {
	baseURL := requireEnv("MEALIE_BASE")
	token := requireEnv("MEALIE_TOKEN")

	now := time.Now()
	weekday := now.Weekday()
	startOfWeek := now.AddDate(0, 0, -int(weekday))
	endOfWeek := startOfWeek.AddDate(0, 0, 6)

	startDate := startOfWeek.Format("2006-01-02")
	endDate := endOfWeek.Format("2006-01-02")

	fmt.Printf("Meal plan: %s → %s\n\n", startDate, endDate)

	entries, err := fetchMealplans(baseURL, token, startDate, endDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching meal plans: %v\n", err)
		os.Exit(1)
	}

	if len(entries) == 0 {
		fmt.Println("No meal plan entries for this week.")
		return
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Date < entries[j].Date
	})

	seen := map[string]bool{}
	var slugs []string
	slugToName := map[string]string{}

	for _, e := range entries {
		slug := ""
		name := e.Title
		if e.Recipe != nil {
			slug = e.Recipe.Slug
			name = e.Recipe.Name
		}
		if slug == "" {
			continue
		}
		day := e.Date
		fmt.Printf("  %s  %-10s  %s\n", day, e.EntryType, name)
		if !seen[slug] {
			seen[slug] = true
			slugs = append(slugs, slug)
			slugToName[slug] = name
		}
	}
	fmt.Println()

	var allIngredients []string
	for _, slug := range slugs {
		r, err := fetchRecipe(baseURL, token, slug)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not fetch recipe %q: %v\n", slug, err)
			continue
		}
		fmt.Printf("--- %s ---\n", r.Name)
		for _, ing := range r.RecipeIngredient {
			line := ingredientLine(ing)
			if line != "" {
				fmt.Printf("  %s\n", line)
				allIngredients = append(allIngredients, line)
			}
		}
		fmt.Println()
	}

	if len(allIngredients) == 0 {
		fmt.Println("No ingredients found.")
		return
	}

	fmt.Println("========================================")
	fmt.Println("  COMBINED SHOPPING LIST")
	fmt.Println("========================================")
	for _, line := range allIngredients {
		fmt.Printf("  • %s\n", line)
	}
	fmt.Printf("\n%d ingredients total across %d recipes\n", len(allIngredients), len(slugs))
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

func fetchMealplans(baseURL, token, startDate, endDate string) ([]planEntry, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	u.Path = "/api/households/mealplans"
	q := u.Query()
	q.Set("start_date", startDate)
	q.Set("end_date", endDate)
	q.Set("perPage", "100")
	u.RawQuery = q.Encode()

	body, err := doGet(u.String(), token)
	if err != nil {
		return nil, err
	}

	var page mealplanPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("parse mealplans: %w", err)
	}
	return page.Items, nil
}

func fetchRecipe(baseURL, token, slug string) (*recipe, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	u.Path = "/api/recipes/" + slug

	body, err := doGet(u.String(), token)
	if err != nil {
		return nil, err
	}

	var r recipe
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("parse recipe: %w", err)
	}
	return &r, nil
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
