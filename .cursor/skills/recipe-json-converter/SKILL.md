---
name: recipe-json-converter
description: Converts recipe-style text or markdown files into normalized schema.org Recipe JSON. Use when the user wants to transform a recipe document, markdown recipe, or loose ingredients-and-steps text into structured Recipe JSON.
---

# Recipe JSON Converter

## Goal

Turn a recipe-like text file into a plain JSON object that uses `schema.org/Recipe` property names.

Default behavior:
- Output plain JSON, not JSON-LD
- Normalize loose recipe text before emitting JSON
- Write the final JSON to the repo's `json/` directory so it can be tracked
- Prefer explicit facts from the source over guessed metadata

## When To Use

Use this skill when:
- The input is a text or markdown recipe file
- The user wants `schema.org/Recipe` JSON
- The recipe has recognizable sections like title, ingredients, steps, notes, yield, or timing

## Output Contract

Default output target: write the generated file into `json/`.

If the source file is `Red Beans.md`, prefer an output path like `json/red-beans.json`.

Unless the user asks otherwise:
- Create or update the JSON file in `json/`
- Return the JSON content or a short confirmation, depending on the user's request
- Do not place generated recipe JSON beside the source markdown file

Use this shape by default:

```json
{
  "name": "",
  "description": "",
  "recipeIngredient": [],
  "recipeInstructions": []
}
```

Add other `schema.org/Recipe` fields only when they are explicit or strongly supported by the source, for example:
- `recipeYield`
- `prepTime`
- `cookTime`
- `totalTime`
- `recipeCategory`
- `recipeCuisine`
- `keywords`
- `suitableForDiet`
- `nutrition`
- `tool`
- `supply`

## Extraction Workflow

1. Read the entire source file.
2. Identify the title, summary, and any labeled sections.
3. Map source content into `Recipe` fields.
4. Normalize wording and structure without changing the recipe's meaning.
5. Write the valid JSON object to `json/<normalized-name>.json`.
6. Emit no extra commentary unless the user asks for explanation.

## Field Mapping

- `name`
  - Use the first non-empty heading or line.

- `description`
  - Use the short descriptive paragraph near the top if present.
  - Keep it concise and human-readable.

- `recipeIngredient`
  - Prefer an array of natural-language ingredient strings.
  - Strip bullets and surrounding whitespace.
  - Each string should read naturally, e.g. `"6 large eggs"`, `"2 tablespoons butter"`, `"4 slices whole wheat bread"`.
  - Use standard cooking units when present (cup, tablespoon, teaspoon, pound, ounce, etc.).
  - Do not use "pieces" as a unit — write `"6 large eggs"` not `"6 pieces large eggs"`. If the original says "pieces", drop it and write the quantity directly before the food.
  - Preserve quantities, units, and ingredient wording from the source unless there is an obvious formatting mistake or unnatural phrasing (like "pieces" used as a unit for countable items).

- `recipeInstructions`
  - Prefer an array of step strings.
  - Remove step numbers from the stored text unless the numbering itself carries meaning.
  - Keep each instruction as one readable step.

- `recipeYield`, `prepTime`, `cookTime`, `totalTime`
  - Include only if the source states them directly or they can be derived from explicit timing text.
  - Durations should use ISO 8601 format like `PT15M` or `PT2H`.

- `recipeCuisine` and `recipeCategory`
  - Include only if directly stated in the source or clearly implied by a descriptive line such as "A Louisiana classic."

- `keywords`
  - Use only explicit tags or a short list of directly supported terms.

- `nutrition`, `tool`, `supply`, `suitableForDiet`
  - Omit unless the source gives concrete information.

## Normalization Rules

- Keep the recipe faithful to the source.
- Do not invent missing times, yield, nutrition, or dietary claims.
- Prefer omission over speculation.
- Preserve ingredient text if structured parsing would lose meaning.
- If a section is unlabeled, infer it only when the format is obvious.
- If the file contains notes, keep them out of the main JSON unless they clearly map to a schema field.

### Ingredient Normalization

Ingredient strings are later parsed by Mealie's NLP ingredient parser, which expects natural-language strings in the form `"<quantity> <unit> <food>, <note>"`. Write ingredients so the parser can extract quantity, unit, and food correctly:

- Use recognized unit names: teaspoon, tablespoon, cup, ounce, pound, gram, kilogram, liter, milliliter, fluid ounce, pint, quart, gallon, pinch, dash, splash, can, bunch, clove, head, serving, sprig, pack.
- For countable items with no measurement unit (eggs, sausage links, bread slices), omit any unit — just write the quantity followed by the food: `"6 large eggs"`, `"4 andouille sausage links"`, `"2 green onions, sliced"`.
- Never use "pieces" as a unit. Drop it entirely.
- Use decimal quantities (`0.5`, `1.5`) or fractions (`1/2`, `1 1/2`) — both are acceptable.
- Put preparation notes and qualifiers after the food, separated by a comma: `"1 pound dried red kidney beans, soaked overnight"`.
- Put parenthetical alternatives at the end: `"1 pound dried red kidney beans (or 2 cans, drained)"`.

## Timing Rules

When explicit durations appear, convert them to ISO 8601:
- `15 minutes` -> `PT15M`
- `1 hour` -> `PT1H`
- `1.5 hours` -> `PT1H30M`
- `2 hours 15 minutes` -> `PT2H15M`

If multiple alternative durations are present, only populate a time field if one default value is clearly intended. Otherwise omit it.

## Quality Bar

Before returning the JSON:
- Confirm required core fields are present when available: `name`, `recipeIngredient`, `recipeInstructions`
- Ensure arrays contain clean strings with no bullet markers
- Ensure JSON is syntactically valid
- Ensure every included optional field is supported by the source text
- Ensure the file is written inside `json/`

## Example

See [examples.md](examples.md) for a worked example using a markdown recipe file.
