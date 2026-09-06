// Package recipedata loads recipes and meal plans from the repository's
// recipes/ and meal-plans/ directories, which are the source of truth.
package recipedata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Recipe is the subset of schema/recipe.cue that the tools need.
type Recipe struct {
	Slug string `json:"-"`

	Name               string   `json:"name"`
	Description        string   `json:"description,omitempty"`
	RecipeIngredient   []string `json:"recipeIngredient"`
	RecipeInstructions []string `json:"recipeInstructions"`
	RecipeYield        string   `json:"recipeYield,omitempty"`
	RecipeCategory     string   `json:"recipeCategory,omitempty"`
	RecipeCuisine      string   `json:"recipeCuisine,omitempty"`
	PrepTime           string   `json:"prepTime,omitempty"`
	CookTime           string   `json:"cookTime,omitempty"`
	TotalTime          string   `json:"totalTime,omitempty"`
}

// MealPlan mirrors schema/mealplan.cue.
type MealPlan struct {
	WeekStart string  `json:"weekStart"`
	Entries   []Entry `json:"entries"`
	Notes     string  `json:"notes,omitempty"`
}

// Entry is a single slot in a meal plan. Exactly one of Recipe or Title is set.
type Entry struct {
	Date     string `json:"date"`
	Meal     string `json:"meal"`
	Recipe   string `json:"recipe,omitempty"`
	Title    string `json:"title,omitempty"`
	Servings int    `json:"servings,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

// Label is what to show for an entry when the recipe itself isn't loaded.
func (e Entry) Label() string {
	if e.Title != "" {
		return e.Title
	}
	return e.Recipe
}

// Store reads recipes and meal plans from a repository root.
type Store struct {
	Root string
}

// Open locates the data root and returns a Store.
//
// The root is the RECIPES_ROOT env var if set, otherwise the nearest ancestor
// of the working directory containing both recipes/ and meal-plans/.
func Open() (*Store, error) {
	if root := os.Getenv("RECIPES_ROOT"); root != "" {
		return &Store{Root: root}, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for {
		if isRoot(dir) {
			return &Store{Root: dir}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("no recipes/ and meal-plans/ directories found above %s (set RECIPES_ROOT)", mustGetwd())
		}
		dir = parent
	}
}

func isRoot(dir string) bool {
	for _, sub := range []string{"recipes", "meal-plans"} {
		if fi, err := os.Stat(filepath.Join(dir, sub)); err != nil || !fi.IsDir() {
			return false
		}
	}
	return true
}

func mustGetwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

// Recipe loads a single recipe by slug.
func (s *Store) Recipe(slug string) (*Recipe, error) {
	path := filepath.Join(s.Root, "recipes", slug+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Recipe
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	r.Slug = slug
	return &r, nil
}

// MealPlan loads the plan for the week beginning on weekStart (YYYY-MM-DD).
func (s *Store) MealPlan(weekStart string) (*MealPlan, error) {
	path := filepath.Join(s.Root, "meal-plans", weekStart+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p MealPlan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	sort.SliceStable(p.Entries, func(i, j int) bool {
		return p.Entries[i].Date < p.Entries[j].Date
	})
	return &p, nil
}

// WeekStart returns the Sunday on or before t, formatted YYYY-MM-DD.
func WeekStart(t time.Time) string {
	return t.AddDate(0, 0, -int(t.Weekday())).Format(DateLayout)
}

// DateLayout is the date format used by filenames and entry dates.
const DateLayout = "2006-01-02"
