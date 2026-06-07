---
name: shopping-list-optimizer
description: >-
  Analyze a meal plan's shopping list against source recipe files and recommend
  ingredient substitutions that reduce the total number of distinct items to buy.
  Then apply the chosen substitutions to the source recipe markdown files. Use
  when the user wants to simplify a shopping list, reduce ingredient variety,
  consolidate a meal plan, or find recipe substitutions.
---

# Shopping List Optimizer

Cross-reference a weekly meal plan's ingredients against the source recipes in
`markdown/` to find substitutions that shrink the shopping list, then apply them.

## Workflow

### 1. Gather context

- Read `shopping.md` (or whatever shopping list the user points to).
- Identify every source recipe referenced in the meal plan.
- Read each source recipe from `markdown/`.

### 2. Analyze for substitution opportunities

Scan for these categories (highest-impact first):

| Category | What to look for |
|---|---|
| **Greens** | Multiple leafy greens (kale, spinach, arugula). Prefer the one already used in the most recipes. Check recipe notes for "or" alternatives. |
| **Seeds & nuts** | Count distinct seed/nut types across all recipes. Combine where they serve the same role (topping, omega-3 source, binder). |
| **Fresh herbs** | Multiple herb bunches bought for small garnish amounts. Parsley is the safest universal sub. Check recipe notes for "or" alternatives. |
| **Citrus** | Lime vs lemon — one usually works for both. Prefer whichever is used in more recipes. |
| **Alliums** | Multiple onion types (red, yellow, white) in small quantities. One type can usually cover all. |
| **Non-dairy milks** | Multiple carton milks (almond, oat, coconut). Pick one for all uses. Keep canned coconut milk separate — it's a different product. |
| **Berries & fruit** | Fresh vs frozen of the same fruit, or multiple berry types used in tiny amounts. Frozen is more versatile and lasts longer. |
| **Overlapping proteins** | Rarely substitutable, but flag if two similar proteins appear (e.g., two white fish). |

### 3. Present recommendations

For each substitution, explain:
- Which ingredient is being replaced and in which recipe.
- What it's being replaced with and why (shared with another recipe, recipe notes allow it, same functional role).
- Net item reduction.

### 4. Apply changes

When the user approves:
- Edit each affected recipe file in `markdown/`.
- Update **both** the INGREDIENTS list and the STEPS text (ingredients are often repeated inline in steps).
- Use `replace_all: true` when the ingredient name appears in both sections.
- Verify each file after editing.

## Guidelines

- Never substitute a protein unless the user explicitly asks.
- Respect "or" alternatives already noted in recipes — these are free wins.
- Prefer eliminating perishables (fresh herbs, berries) over pantry items.
- When consolidating seeds/nuts, adjust quantities so total volume stays reasonable.
- Don't change spices — they're pantry staples with long shelf life.
- Keep the recipe's character intact; don't turn a Mediterranean dish into a curry.
