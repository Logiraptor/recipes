# AGENTS.md

This document provides essential information for agents working in this codebase to understand the project structure, commands, and patterns.

## Storage Model

The git repo itself is the database: `recipes/` and `meal-plans/` are the
source of truth, validated by the CUE schemas in `schema/`. There is no
external recipe service — the tools under `cmd/` read these directories
directly, over the filesystem, with no credentials or network access.

## Project Overview

A Go project for managing recipes and weekly meal plans as JSON files, with three tools:
1. `trmnl-recipe` - Picks the recipe for the current meal slot and sends it to a TRMNL webhook
2. `mealplan-ingredients` - Prints the week's meal plan and a combined shopping list
3. `recipes-web` - Read-only website browsing recipes and meal plans (daisyUI/Tailwind UI)

## Code Organization

### Directory Structure
- `recipes/` - Recipe files in JSON format (schema.org Recipe subset), one per
  file. The filename (minus `.json`) is the recipe's **slug**.
- `meal-plans/` - Weekly meal plans in JSON, one file per week named after the
  week's start date (Sunday), e.g. `meal-plans/2025-06-01.json`. Entries
  reference recipes by slug.
- `schema/` - CUE schemas (`#Recipe`, `#MealPlan`) plus `validate.sh`, which
  vets every data file and checks slug/filename/cross-reference integrity.
  Run `./schema/validate.sh` after editing any data file.
  Requires `cue` (`go install cuelang.org/go/cmd/cue@latest`) and `jq`.
- `internal/recipedata/` - Shared loader for `recipes/` and `meal-plans/`.
  Both commands go through it; add new data access here, not in `cmd/`.
- `cmd/trmnl-recipe/` - Source code for the TRMNL recipe webhook tool
- `cmd/mealplan-ingredients/` - Source code for the meal plan ingredients tool
- `cmd/recipes-web/` - Web server; `templates/*.html` are embedded via `embed.FS`.
  Styling comes from daisyUI (MIT) on top of Tailwind, loaded from a CDN, so
  there is no CSS build step and no hand-rolled design system.
- `deploy/` - Deployment configuration files

## Build and Run

```bash
# Build both commands
go build ./...

# Run the shopping list for the current week (or a specific week)
go run ./cmd/mealplan-ingredients
go run ./cmd/mealplan-ingredients -week 2025-06-01

# Serve the website at http://localhost:8080
go run ./cmd/recipes-web

# Build with CGO disabled for smaller binaries (as in Dockerfile)
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o trmnl-recipe ./cmd/trmnl-recipe
```

### Docker Build
```bash
docker build -t trmnl-recipe .
docker build -f Dockerfile.web -t recipes-web .
docker run --rm -p 8080:8080 recipes-web
```

The image bakes `recipes/` and `meal-plans/` into `/data` and sets
`RECIPES_ROOT=/data`, so a rebuild is how new recipes reach the device.

## Key Components and Functionality

### Data Loader (`internal/recipedata`)
- `Open()` finds the data root: `RECIPES_ROOT` if set, otherwise the nearest
  ancestor directory containing both `recipes/` and `meal-plans/`
- `Store.Recipe(slug)` and `Store.MealPlan(weekStart)` load and decode files;
  a missing file surfaces as `fs.ErrNotExist`
- `WeekStart(t)` returns the Sunday on or before `t`

### TRMNL Recipe Tool (`trmnl-recipe`)
- Loads this week's meal plan and picks today's entry for the current meal slot
- Selects the meal type (breakfast, lunch, or dinner) based on current time
- Formats recipe data for TRMNL webhook payload using a template file
- Truncates large payloads to fit within TRMNL's 2000-byte limit
- Sends formatted recipe data to configured webhook URL
- A missing meal-plan file is not an error: it pushes the empty state

### Web Tool (`recipes-web`)
- Routes: `/` (searchable recipe grid), `/recipes/{slug}`, `/plan?week=YYYY-MM-DD`, `/healthz`
- Reads from disk on every request, so edits to `recipes/` show up on refresh locally
- Each page template defines a `content` block layered on `templates/layout.html`

### Mealplan Ingredients Tool (`mealplan-ingredients`)
- Loads the weekly meal plan and every recipe it references
- Prints the plan, per-recipe ingredients, and a combined shopping list
- Skips duplicate recipes so each is listed once

## Key Patterns and Conventions

### File Handling
- Recipe files must be valid JSON matching `schema/recipe.cue`
- Ingredients are plain natural-language strings, quantity first
  (`"2 tablespoons olive oil"`); no structured quantity/unit/food objects
- Instructions are plain strings, one step per entry

### Environment Variables
- `RECIPES_ROOT` - Data root override (optional; both tools auto-detect it
  by walking up from the working directory)
- `TRMNL_WEBHOOK_URL` - URL to send the TRMNL webhook payload (trmnl-recipe only)
- `ADDR` - Listen address for `recipes-web` (default `:8080`; `-addr` flag overrides)

### Error Handling
- Tools exit with non-zero status codes on failure
- Error messages are written to stderr; `trmnl-recipe` uses structured `slog`

### Payload Truncation
The trmnl-recipe tool includes truncation logic for recipes that exceed the 2000-byte TRMNL limit:
1. Instructions are truncated first
2. Ingredients are truncated second
3. Additional fields (TotalTime, PrepTime, RecipeYield) are removed as needed
4. If still over limit, instructions are removed entirely and a message is sent instead

## Important Gotchas and Non-Obvious Patterns

1. **Data root discovery**: Running a tool from outside the repo fails unless
   `RECIPES_ROOT` is set. In the container it is always `/data`.

2. **Filename is the slug**: `meal-plans/` entries reference recipes by
   basename. Renaming a recipe file breaks the plans that reference it —
   `./schema/validate.sh` catches this.

3. **Week files are Sunday-anchored**: `meal-plans/<sunday>.json`, and
   `weekStart` inside the file must match the filename.

4. **Time-Based Meal Selection**: trmnl-recipe uses time-based logic:
   - Breakfast: 5 AM - 10:59 AM
   - Lunch: 11 AM - 1:59 PM
   - Dinner: 2 PM and later

5. **Template-Based Output**: `trmnl-template.html` defines the TRMNL
   rendering and uses Liquid-like syntax for variable substitution. Its merge
   variable names must stay in sync with `mergeVariables` in `main.go`.

6. **Deploys are data deploys**: since the data is baked into the image, editing
   `recipes/` or `meal-plans/` requires an image rebuild to take effect on device.

## Testing Approach

There are no test files yet. The tools can be exercised locally:
`go run ./cmd/mealplan-ingredients -week <date>`, or `trmnl-recipe` with
`TRMNL_WEBHOOK_URL` pointed at a local HTTP server.

## Deployment

The Dockerfile:
1. Builds `trmnl-recipe` with CGO disabled for a small static binary
2. Copies `recipes/` and `meal-plans/` into `/data`
3. Uses a distroless base image to minimize attack surface

`deploy/trmnl-recipe-cronjob.example.yaml` runs it as a CronJob; the only
secret it needs is `TRMNL_WEBHOOK_URL`.

### Images and CI
Two GitHub Actions workflows publish multi-arch images to GHCR on pushes to
`main` (and via `workflow_dispatch`), tagged `main` and `sha-<sha>`:
- `.github/workflows/publish.yml` → `ghcr.io/<repo>` from `Dockerfile` (trmnl-recipe)
- `.github/workflows/publish-web.yml` → `ghcr.io/<repo>-web` from `Dockerfile.web` (recipes-web)
