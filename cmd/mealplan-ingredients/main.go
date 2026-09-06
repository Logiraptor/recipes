// Command mealplan-ingredients prints the week's meal plan and a combined
// shopping list, reading directly from meal-plans/ and recipes/.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/patrickoyarzun/recipes/internal/recipedata"
)

func main() {
	week := flag.String("week", "", "week start date (YYYY-MM-DD, Sunday); defaults to the current week")
	flag.Parse()

	if err := run(*week); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(week string) error {
	store, err := recipedata.Open()
	if err != nil {
		return err
	}

	if week == "" {
		week = recipedata.WeekStart(time.Now())
	}
	start, err := time.Parse(recipedata.DateLayout, week)
	if err != nil {
		return fmt.Errorf("invalid week %q: %w", week, err)
	}
	end := start.AddDate(0, 0, 6)

	plan, err := store.MealPlan(week)
	if err != nil {
		return fmt.Errorf("load meal plan: %w", err)
	}

	fmt.Printf("Meal plan: %s → %s\n\n", week, end.Format(recipedata.DateLayout))
	if len(plan.Entries) == 0 {
		fmt.Println("No meal plan entries for this week.")
		return nil
	}

	var slugs []string
	recipes := map[string]*recipedata.Recipe{}
	for _, e := range plan.Entries {
		if e.Recipe == "" || recipes[e.Recipe] != nil {
			continue
		}
		r, err := store.Recipe(e.Recipe)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load recipe %q: %v\n", e.Recipe, err)
			continue
		}
		recipes[e.Recipe] = r
		slugs = append(slugs, e.Recipe)
	}

	for _, e := range plan.Entries {
		label := e.Label()
		if r := recipes[e.Recipe]; r != nil {
			label = r.Name
		}
		fmt.Printf("  %s  %-10s  %s\n", e.Date, e.Meal, label)
	}
	fmt.Println()

	var all []string
	for _, slug := range slugs {
		r := recipes[slug]
		fmt.Printf("--- %s ---\n", r.Name)
		for _, ing := range r.RecipeIngredient {
			fmt.Printf("  %s\n", ing)
			all = append(all, ing)
		}
		fmt.Println()
	}

	if len(all) == 0 {
		fmt.Println("No ingredients found.")
		return nil
	}

	fmt.Println("========================================")
	fmt.Println("  COMBINED SHOPPING LIST")
	fmt.Println("========================================")
	for _, line := range all {
		fmt.Printf("  • %s\n", line)
	}
	fmt.Printf("\n%d ingredients total across %d recipes\n", len(all), len(slugs))
	return nil
}
