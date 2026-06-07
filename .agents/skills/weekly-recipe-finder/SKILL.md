---
name: weekly-recipe-finder
description: >-
  Find a week of highly-rated recipes for a given ingredient or constraint,
  matched to the user's cooking comfort zone, mixing existing repo recipes with
  new ones found online (roughly 60/40 reuse vs new), with ingredients reused
  across recipes to simplify shopping. Writes them as markdown and schema.org
  JSON. Use when the user wants to plan a week of meals, find new recipes for
  ingredients they have, or build a meal plan around a protein, diet, or
  cooking method.
---

# Weekly Recipe Finder

Find a cohesive set of highly-rated recipes for the week — built around the
user's available ingredients and constraints, matched to their demonstrated
cooking comfort zone, and chosen so ingredients overlap to keep the shopping
trip simple. Recipes can come from two places: **existing recipes already in
this repo** (`markdown/`) and **new, real recipes found online**. Aim for a mix
of roughly **60% reused / 40% new** when enough qualifying repo recipes exist.
Output both markdown (in `markdown/`) and schema.org Recipe JSON (in `json/`).

This is a large, multi-step task. Use subagents to parallelize the work and
keep the orchestrator in control.

## Inputs to confirm

Before starting, identify from the request:
- **Core ingredient(s)** the user has (e.g., chicken thighs and breast).
- **Constraints / priorities** (e.g., Instant Pot, anti-inflammatory, kid-friendly, time limits).
- **How many recipes** (default 5 dinners if unspecified).

## Workflow

### 1. Learn the user's comfort zone and inventory existing recipes (do this first)

Read the existing recipes from `markdown/` (prioritize ones matching the
target ingredient or method) to do two things at once:

1. **Build a comfort-zone profile** (table below).
2. **Inventory reuse candidates** — note which existing repo recipes already
   satisfy the core ingredient(s) and constraints. These are eligible to be
   reused in the weekly set, so flag their title, method, cuisine, and protein.

Capture for the profile:

| Dimension | What to extract |
|---|---|
| **Methods** | Instant Pot, one-pot, sheet pan, skillet, stir-fry, braise, etc. |
| **Cuisines** | Which flavor families recur (Indian, Mediterranean, Cajun, Asian). |
| **Anti-inflammatory staples** | turmeric, ginger, garlic, coconut milk, olive oil, leafy greens, tomatoes, beans/lentils. |
| **Techniques used** | searing, sautéing aromatics, pressure cooking, blending sauces, baking. |
| **Constraints** | servings (usually 4-6), cook times (20-40 min), toddler/kid considerations. |

Summarize the profile back to the user before searching, and list the existing
repo recipes that already qualify as reuse candidates.

### 2. Research new recipes online (parallel subagents)

Only research enough *new* recipes to fill the gap left after reuse — roughly
**40% of the weekly set** (e.g., 2 new for a 5-recipe week), plus a few extra
candidates for selection flexibility. If too few repo recipes qualify for reuse,
research more new ones to make up the difference.

Split the search across **2-4 parallel `researcher` subagents** by category
(e.g., Instant Pot / Mediterranean / Asian-Indian / stir-fry). Give each agent
the SAME hard requirements:

- Use the core ingredient(s).
- Match the constraints and the comfort-zone profile from step 1.
- **Only real recipes from reputable sites** with a working source URL —
  never invent a recipe. Good sources: budgetbytes, wellplated, pipingpotcurry,
  recipetineats, themediterraneandish, seriouseats, nytimes cooking,
  ambitiouskitchen, halfbakedharvest, cookwithmanali.
- **Filter for highly rated**: 4.5+ stars with a meaningful number of reviews.
  Drop SEO/aggregator spam and low-review-count results.
- Return per recipe: title, source URL, rating + review count, servings,
  total time, full ingredient list with quantities, and concise numbered steps.
- Write each agent's findings to a `research-<category>.md` file.

Example launch (PARALLEL mode, concurrency 3):
- Task A → `researcher`, output `research-instantpot.md`
- Task B → `researcher`, output `research-mediterranean.md`
- Task C → `researcher`, output `research-asian.md`

### 3. Select the weekly set (maximize ingredient reuse)

From all candidates — **both reuse candidates from the repo and newly researched
online recipes** — pick the requested number of recipes. Target roughly a
**60% reused / 40% new** split when enough qualifying repo recipes exist; if not,
lean more on new recipes and note why. Optimize for:
- **Reuse first** — prefer an existing repo recipe over a new one when both fit
  equally well; it's already in your format and you've made it before.
- **Priority match** — honor the user's top constraint (e.g., majority Instant Pot).
- **Variety** — don't pick five near-identical dishes; vary method and cuisine.
- **Ingredient overlap** — this is the shopping-trip win. Favor recipes that
  share a backbone: the core protein, aromatics (onion/garlic/ginger), canned
  goods (coconut milk, tomatoes), citrus, greens, and a shared spice set.
- **Rating** — prefer higher-rated, well-reviewed options when choosing between
  similar dishes.

Present the selection as a table (recipe, source, rating, method, protein, and
whether it's **reused** or **new**) plus a short note on the shared-ingredient
backbone and the reuse/new split.

### 4. Write the markdown recipes

For **reused** recipes, the markdown already exists — leave it as-is (don't
rewrite or duplicate it). Only write markdown for the **new** selected recipes,
to `markdown/<Title>.md` following the existing markdown format in the repo:
- Title line, then a one-line description (include source + rating).
- `Servings:`, `Prep Time:`, `Cook Time:`, `Total Time:` lines.
- `INGREDIENTS` section with `• ` bullets, natural-language quantities.
- `STEPS` section with numbered steps.
- Optional `NOTES` section (substitutions, kid-friendly tips, greens add-ins).

### 5. Convert to JSON (subagent)

Reused recipes should already have JSON in `json/` — verify it exists and skip
re-converting them. Delegate to the `recipe-json-converter` subagent (or skill)
to convert only the **new** markdown files into schema.org Recipe JSON in
`json/`, following that
skill's conventions (kebab-case filenames, no "pieces" unit, ISO 8601 durations,
`tool`/`recipeCuisine` where clear).

### 6. Verify

- Confirm every JSON file for **new** recipes is valid (e.g.,
  `python3 -c "import json; json.load(...)"`).
- Confirm each **reused** recipe has both its markdown and JSON already present.
- Confirm markdown and JSON counts match across the full weekly set.
- Report the final table (marking reused vs new), the reuse/new split, and the
  shared-ingredient shopping summary.

## Guidelines

- **Never invent recipes.** Every *new* recipe must trace to a real, working URL
  with a verifiable rating; reused recipes must come from the repo's existing
  `markdown/` files. If research can't find enough qualifying recipes, say so
  rather than filling the gap with made-up dishes.
- **Aim for ~60% reuse / 40% new** when enough existing repo recipes qualify.
  Reuse is a feature, not a fallback — it favors recipes the user already knows
  and that are already in the repo's format. Only lean more heavily on new
  recipes when too few repo recipes fit the criteria (and note why).
- Track progress with the `todo` tool for this multi-step task.
- Keep the orchestrator in control; subagents do research and conversion only.
- Respect the comfort zone — don't pick techniques or equipment the user has
  never used unless they ask.
- Clean up `research-*.md` scratch files at the end, or leave them if the user
  may want to review the dropped candidates.
- Offer follow-ups: a quantity-merged shopping list, or syncing to Mealie.
