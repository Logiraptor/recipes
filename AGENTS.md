# AGENTS.md

This document provides essential information for agents working in this codebase to understand the project structure, commands, and patterns.

## Project Overview

This is a Go-based project that interacts with the Mealie recipe management system. It includes three primary tools:
1. `recipes` - Main application for syncing JSON recipe files to Mealie
2. `trmnl-recipe` - A command-line tool that fetches today's meal plan from Mealie and sends it to a TRMNL webhook
3. `mealplan-ingredients` - A command-line tool that generates a combined shopping list from the weekly meal plan

## Code Organization

### Directory Structure
- `json/` - Contains recipe files in JSON format (schema.org Recipe format)
- `markdown/` - Contains recipe files in Markdown format
- `mealie/` - Contains Mealie API client code (generated from OpenAPI spec)
- `cmd/trmnl-recipe/` - Source code for the TRMNL recipe webhook tool
- `cmd/mealplan-ingredients/` - Source code for the meal plan ingredients tool 
- `deploy/` - Deployment configuration files

### Go Modules
The project uses Go modules with dependencies managed by `go.mod`. It includes:
- `github.com/oapi-codegen/runtime` for OpenAPI code generation
- Mealie API client generated from OpenAPI specification

## Build and Deployment

### Build Commands
```bash
# Build the main application
go build -o recipes ./main.go

# Build the trmnl-recipe command
go build -o trmnl-recipe ./cmd/trmnl-recipe/main.go

# Build the mealplan-ingredients command
go build -o mealplan-ingredients ./cmd/mealplan-ingredients/main.go

# Build with CGO disabled for smaller binaries (as in Dockerfile)
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o trmnl-recipe ./cmd/trmnl-recipe
```

### Docker Build
The project includes a Dockerfile for containerized deployment:
```bash
docker build -t trmnl-recipe .
```

## Key Components and Functionality

### Main Application (`recipes`)
- Syncs JSON recipe files to Mealie via the API
- Supports syncing single files or directories of files
- Uses the Mealie API client to create/update recipes
- Processes recipe ingredients through Mealie's NLP parser to normalize them

### TRMNL Recipe Tool (`trmnl-recipe`)
- Fetches today's meal plan from Mealie API
- Selects the appropriate meal type (breakfast, lunch, or dinner) based on current time
- Formats recipe data for TRMNL webhook payload using a template file
- Truncates large payloads to fit within TRMNL's 2000-byte limit
- Sends formatted recipe data to configured webhook URL

### Mealplan Ingredients Tool (`mealplan-ingredients`)
- Fetches the weekly meal plan from Mealie API
- Generates a combined shopping list from all ingredients in the meal plan
- Groups ingredients by recipe and formats them for display

### Mealie API Integration
The project uses an OpenAPI client that's generated from the Mealie API specification:
- `mealie/client.gen.go` is auto-generated code
- `mealie/oapi-codegen.yaml` configures the generation
- The client is used for all API interactions with Mealie

## Key Patterns and Conventions

### File Handling
- Recipe files are expected to be valid JSON following the schema.org Recipe specification
- Files can be processed individually or as a directory of files
- File names are used to determine recipe names for search operations

### Environment Variables
All tools require specific environment variables:
- `MEALIE_BASE` - Base URL for the Mealie instance (required)
- `MEALIE_TOKEN` - Bearer token for Mealie API authentication
- `TRMNL_WEBHOOK_URL` - URL to send the TRMNL webhook payload (only for trmnl-recipe)

### Error Handling
- All tools exit with non-zero status codes on failure
- Error messages are written to stderr for debugging
- HTTP error responses from Mealie API are logged with status codes and response bodies

### Ingredient Processing
- Ingredients are parsed through Mealie's NLP ingredient parser for normalization (in recipes tool)
- Special handling to avoid sending invalid food/unit references that could cause 500 errors

### Payload Truncation
The trmnl-recipe tool includes sophisticated payload truncation logic to handle cases where the recipe data exceeds the 2000-byte limit for TRMNL webhooks:
1. Instructions are truncated first
2. Ingredients are truncated second
3. Additional fields (TotalTime, PrepTime, RecipeYield) are removed as needed
4. If still over limit, instructions are completely removed and a message is sent instead

## Testing Approach

There are no explicit test files in the current view of the codebase, but the tools:
1. Can be tested by running them with actual Mealie instances
2. Have unit test-like logic in their error handling and validation
3. Are designed to work with the Mealie API through direct HTTP calls

## Important Gotchas and Non-Obvious Patterns

1. **Recipe File Format**: Recipe files must be valid JSON following the schema.org Recipe specification with a "name" field to be processed.

2. **Ingredient Parsing**: The recipes tool leverages Mealie's NLP ingredient parser for normalization, but has special handling to avoid sending invalid food/unit references that could cause 500 errors.

3. **Payload Size Management**: The trmnl-recipe tool has a complex truncation strategy that prioritizes sending key recipe information while ensuring the payload fits within TRMNL limits.

4. **API Rate Limiting**: The tools make direct HTTP calls to the Mealie API, so they don't include any built-in rate limiting or retry logic.

5. **Generated Client Code**: The `mealie/client.gen.go` file is auto-generated from the OpenAPI spec and should not be modified directly.

6. **Time-Based Meal Selection**: The trmnl-recipe tool uses time-based logic to determine meal type:
   - Breakfast: 5 AM - 10:59 AM
   - Lunch: 11 AM - 1:59 PM  
   - Dinner: 2 PM and later

7. **Multiple Meal Plan Formats**: The trmnl-recipe tool can handle both array-based and paginated meal plan responses from the Mealie API.

8. **Template-Based Output**: The trmnl-recipe tool uses a template file (`trmnl-template.html`) that defines the formatting of the TRMNL webhook payload. The template uses a Liquid-like syntax for variable substitution.

9. **Weekly Meal Plan Processing**: The mealplan-ingredients tool fetches a full weekly meal plan and generates combined shopping lists across all recipes.

10. **Week Start Calculation**: The mealplan-ingredients tool calculates the current week's start date based on the current day, assuming Sunday as the first day of the week.

11. **Duplicate Recipe Handling**: The mealplan-ingredients tool avoids duplicate processing of the same recipe by tracking seen slugs.

## Deployment

The project includes a Dockerfile that:
1. Builds the application with CGO disabled for smaller binaries
2. Uses a distroless base image to minimize attack surface
3. Sets up the binary as the entrypoint

Deployment configuration examples are in `deploy/` directory.