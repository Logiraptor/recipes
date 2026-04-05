package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/patrickoyarzun/recipes/mealie"
)

func main() {
	baseURL := envOr("MEALIE_BASE", "https://mealie.home.poyarzun.io")
	token := requireEnv("MEALIE_TOKEN")
	jsonDir := "json"
	if len(os.Args) > 1 {
		jsonDir = os.Args[1]
	}

	client, err := mealie.NewClient(baseURL, mealie.WithRequestEditorFn(bearerAuth(token)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: creating client: %v\n", err)
		os.Exit(1)
	}

	entries, err := os.ReadDir(jsonDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: directory %q: %v\n", jsonDir, err)
		os.Exit(1)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, filepath.Join(jsonDir, e.Name()))
		}
	}
	if len(files) == 0 {
		fmt.Printf("No .json files found in %s\n", jsonDir)
		return
	}

	ctx := context.Background()
	var created, updated, failed int
	for _, f := range files {
		name, err := recipeName(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL %s — %v\n", f, err)
			failed++
			continue
		}

		slug, exists, err := findRecipe(ctx, client, name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL %s — search error: %v\n", f, err)
			failed++
			continue
		}

		if exists {
			if err := updateRecipe(ctx, client, f, slug); err != nil {
				fmt.Fprintf(os.Stderr, "FAIL %s — update error: %v\n", f, err)
				failed++
			} else {
				fmt.Printf("UPDATED  %s (%s)\n", name, f)
				updated++
			}
		} else {
			if err := createRecipe(ctx, client, f); err != nil {
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

func bearerAuth(token string) mealie.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
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

func findRecipe(ctx context.Context, client *mealie.Client, name string) (slug string, exists bool, err error) {
	perPage := 50
	params := &mealie.GetAllApiRecipesGetParams{
		Search:  &name,
		PerPage: &perPage,
	}

	resp, err := client.GetAllApiRecipesGet(ctx, params)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false, fmt.Errorf("search HTTP %d: %s", resp.StatusCode, truncate(body, 200))
	}

	var result mealie.PaginationBaseRecipeSummary
	if err := json.Unmarshal(body, &result); err != nil {
		return "", false, fmt.Errorf("parse search response: %w", err)
	}

	for _, item := range result.Items {
		if item.Name != nil && *item.Name == name {
			if item.Slug != nil {
				return *item.Slug, true, nil
			}
		}
	}
	return "", false, nil
}

func createRecipe(ctx context.Context, client *mealie.Client, path string) error {
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
	normalizeRecipe(recipe)

	recipeJSON, err := json.Marshal(recipe)
	if err != nil {
		return err
	}

	body := mealie.ScrapeRecipeData{
		Data: string(recipeJSON),
	}

	resp, err := client.CreateRecipeFromHtmlOrJsonApiRecipesCreateHtmlOrJsonPost(ctx, nil, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var slug string
	if err := json.Unmarshal(respBody, &slug); err != nil {
		return fmt.Errorf("parse create response: %w", err)
	}

	if err := patchIngredients(ctx, client, path, slug); err != nil {
		return fmt.Errorf("ingredient patch: %w", err)
	}
	return nil
}

func updateRecipe(ctx context.Context, client *mealie.Client, path string, slug string) error {
	return patchIngredients(ctx, client, path, slug)
}

func patchIngredients(ctx context.Context, client *mealie.Client, path string, slug string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var recipe map[string]any
	if err := json.Unmarshal(raw, &recipe); err != nil {
		return err
	}

	ingredients, err := resolveIngredients(ctx, client, recipe)
	if err != nil {
		return err
	}
	if ingredients == nil {
		return nil
	}

	patch := map[string]any{
		"recipeIngredient": ingredients,
	}
	patchBody, err := json.Marshal(patch)
	if err != nil {
		return err
	}

	resp, err := client.PatchOneApiRecipesSlugPatchWithBody(ctx, slug, nil, "application/json", bytes.NewReader(patchBody))
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

func parseIngredients(ctx context.Context, client *mealie.Client, ingredients []string) ([]mealie.ParsedIngredient, error) {
	parser := mealie.Nlp
	body := mealie.IngredientsRequest{
		Parser:      &parser,
		Ingredients: ingredients,
	}

	resp, err := client.ParseIngredientsApiParserIngredientsPost(ctx, nil, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("parser HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var results []mealie.ParsedIngredient
	if err := json.Unmarshal(respBody, &results); err != nil {
		return nil, fmt.Errorf("parse parser response: %w", err)
	}
	return results, nil
}

func normalizeRecipe(recipe map[string]any) {
	if cat, ok := recipe["recipeCategory"]; ok {
		if s, ok := cat.(string); ok {
			recipe["recipeCategory"] = []string{s}
		}
	}

	if instRaw, ok := recipe["recipeInstructions"]; ok {
		if arr, ok := instRaw.([]any); ok {
			normalized := make([]any, len(arr))
			for i, v := range arr {
				switch v := v.(type) {
				case string:
					normalized[i] = map[string]string{"text": v}
				default:
					normalized[i] = v
				}
			}
			recipe["recipeInstructions"] = normalized
		}
	}
}

// resolveIngredients parses raw ingredient strings via the Mealie NLP parser
// and returns structured ingredient objects ready for PATCH. Returns nil if
// the recipe has no string ingredients to resolve.
func resolveIngredients(ctx context.Context, client *mealie.Client, recipe map[string]any) ([]mealie.RecipeIngredientInput, error) {
	raw, ok := recipe["recipeIngredient"]
	if !ok {
		return nil, nil
	}

	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		return nil, nil
	}

	var strs []string
	for _, v := range arr {
		s, ok := v.(string)
		if !ok {
			return nil, nil
		}
		strs = append(strs, s)
	}

	parsed, err := parseIngredients(ctx, client, strs)
	if err != nil {
		return nil, fmt.Errorf("ingredient parsing: %w", err)
	}

	resolved := make([]mealie.RecipeIngredientInput, len(parsed))
	for i, p := range parsed {
		resolved[i] = toIngredientInput(p.Ingredient)
	}
	return resolved, nil
}

// toIngredientInput converts a parser output ingredient to an input ingredient,
// dropping food/unit sub-objects that lack a valid ID (the NLP parser often
// returns them with empty IDs, which causes Mealie's PATCH to 500).
func toIngredientInput(out mealie.RecipeIngredientOutput) mealie.RecipeIngredientInput {
	inp := mealie.RecipeIngredientInput{
		Display:      out.Display,
		Note:         out.Note,
		OriginalText: out.OriginalText,
		Quantity:     out.Quantity,
		ReferenceId:  out.ReferenceId,
		Title:        out.Title,
	}
	if out.Food != nil && out.Food.Id != "" {
		inp.Food = &mealie.IngredientFoodInput{
			Id:   out.Food.Id,
			Name: out.Food.Name,
		}
	} else if out.Food != nil && out.Food.Name != "" {
		inp.Note = &out.Food.Name
	}
	if out.Unit != nil && out.Unit.Id != "" {
		inp.Unit = &mealie.IngredientUnitInput{
			Id:   out.Unit.Id,
			Name: out.Unit.Name,
		}
	}
	return inp
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
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
