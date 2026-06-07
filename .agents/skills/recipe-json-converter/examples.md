# Example

## Source Pattern

Example source file:

```md
Red Beans and Rice with Andouille Sausage
A Louisiana Monday-night classic - smoky, hearty, and even better reheated the next day.

INGREDIENTS
- 1 pounds dried red kidney beans (or 2 cans, drained)
- 12 ounces andouille sausage, sliced into rounds

STEPS
1. Soak beans overnight if using dried beans.
2. Brown the sausage.
3. Simmer until creamy.
```

## Default Output

Preferred output path:

```text
json/red-beans-and-rice-with-andouille-sausage.json
```

```json
{
  "name": "Red Beans and Rice with Andouille Sausage",
  "description": "A Louisiana Monday-night classic - smoky, hearty, and even better reheated the next day.",
  "recipeCuisine": "Louisiana",
  "recipeIngredient": [
    "1 pounds dried red kidney beans (or 2 cans, drained)",
    "12 ounces andouille sausage, sliced into rounds"
  ],
  "recipeInstructions": [
    "Soak beans overnight if using dried beans.",
    "Brown the sausage.",
    "Simmer until creamy."
  ]
}
```

## Ingredient Normalization

Given source ingredient lines like:

```md
- 6 pieces large eggs
- 4 pieces slices whole wheat bread
- 2 tablespoons butter
```

Normalize to natural-language strings the Mealie parser can handle:

```json
"recipeIngredient": [
  "6 large eggs",
  "4 slices whole wheat bread",
  "2 tablespoons butter"
]
```

Drop "pieces" — it is not a cooking unit. Write countable items as `"<qty> <food>"`.

## Notes

- The output stays as plain JSON.
- The generated file should be written into the repo's `json/` directory.
- `recipeCuisine` is included here because the description explicitly identifies the dish as Louisiana.
- If the source did not clearly support `recipeCuisine`, omit it.
- Ingredient strings should be natural language that Mealie's NLP parser can decompose into quantity, unit, and food.
