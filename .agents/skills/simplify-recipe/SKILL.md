---
name: simplify-recipe
description: >-
  Simplify recipe JSON files in recipes/ to be more realistic for busy parents with
  young children. Use when the user wants to make recipes easier, faster,
  more kid-friendly, reduce ingredient counts, or cut prep steps.
---

# Simplify Recipe

Rewrite recipe JSON files in `recipes/` to be practical for a parent cooking while managing small children. The goal is fewer ingredients, fewer steps, less fussy prep, and kid-safe results — while keeping the recipe recognizably the same dish.

## Simplification Rules

Apply all that are relevant:

### Reduce prep friction
- Remove anything that requires fine knife work (mincing garlic, dicing onions, chopping herbs). Prefer tearing by hand, slicing, or dropping the ingredient entirely.
- Remove fresh ginger grating, zesting, and similar fiddly prep.
- If an aromatic (garlic, onion, shallot) is a minor flavor accent and not the star, cut it.

### Collapse steps
- Merge any steps that use the same pan/pot/blender into a single step.
- A weekday breakfast should be 1-2 steps max. Dinners can be 2-3.
- Remove "sauté aromatics" as a separate step — fold into the main cook step or drop.

### Cut ingredient count
- Target 6-8 ingredients max per recipe.
- Remove garnish-only ingredients (fresh parsley, cilantro, microgreens) unless they define the dish.
- Remove "nice to have" seeds/toppings (hemp seeds, pepitas) that require buying a specialty item.
- Keep ingredients that carry real nutritional or flavor weight.

### Make kid-safe
- Remove or flag choking hazards for toddlers: whole nuts, large seeds, popcorn, whole grapes, raw carrots. Prefer nut butters over chopped nuts, or drop nuts entirely for kids under 3.
- Avoid strong spices that kids reject. Keep mild warming spices (cinnamon, mild turmeric). Drop anything with heat (cayenne, chili flakes).
- Avoid honey for children under 1 (botulism risk) — note maple syrup as a swap if relevant.

### Scale and serve practically
- If the original serves 1, consider scaling up so a parent can share with kids without making a separate meal.
- Convert impractical formats: smoothie bowls become drinkable smoothies (one-handed, portable), deconstructed dishes become one-bowl meals.
- Note toddler serving tips in NOTES when helpful (e.g., "pour into a straw cup").

### Preserve identity
- Keep the core flavor profile and main protein/grain/produce intact.
- Keep the recipe name recognizable — small rename is fine, complete rebrand is not.
- Preserve the nutritional intent (anti-inflammatory, high-protein, etc.) as much as possible.

## Process

1. Read all target recipe files.
2. For each recipe, identify simplifications using the rules above.
3. Edit each file in place, preserving the schema.org Recipe shape (`name`, `description`, `recipeIngredient`, `recipeInstructions`, etc.).
4. Update `description` to reflect the simplified version.
5. Add any relevant parent/kid tips to the final instruction or `description`.
6. Summarize changes to the user: what was removed, what was changed, and why.

## Example

**Before** (10 ingredients, 4 steps, requires mincing and dicing):

```json
{
  "name": "Smoked Salmon & Greens Scramble",
  "recipeIngredient": [
    "3 large eggs",
    "60 grams smoked salmon",
    "1 cup baby spinach",
    "1/3 avocado, diced",
    "1 tablespoon extra-virgin olive oil",
    "2 garlic cloves, minced",
    "1/3 yellow onion, thinly sliced",
    "1/2 cup cherry tomatoes, halved",
    "1 tablespoon fresh parsley, chopped",
    "1/4 teaspoon black pepper"
  ],
  "recipeInstructions": [
    { "@type": "HowToStep", "text": "Warm oil. Add onion and garlic, saute until softened." },
    { "@type": "HowToStep", "text": "Add tomatoes and spinach. Stir until wilted." },
    { "@type": "HowToStep", "text": "Whisk eggs with pepper, pour in, stir until just set." },
    { "@type": "HowToStep", "text": "Fold in salmon. Top with avocado and parsley." }
  ]
}
```

**After** (5 ingredients, 2 steps, no knife work):

```json
{
  "name": "Smoked Salmon & Spinach Scramble",
  "recipeIngredient": [
    "4 large eggs",
    "60 grams smoked salmon",
    "1 cup baby spinach",
    "1/3 avocado, sliced",
    "1 tablespoon extra-virgin olive oil"
  ],
  "recipeInstructions": [
    { "@type": "HowToStep", "text": "Warm oil. Toss in spinach, stir until wilted. Crack in eggs and scramble until just set. Tear salmon into the pan." },
    { "@type": "HowToStep", "text": "Plate and top with avocado. Serve with toast. Bumped to 4 eggs so there's enough to share with a toddler." }
  ]
}
```
