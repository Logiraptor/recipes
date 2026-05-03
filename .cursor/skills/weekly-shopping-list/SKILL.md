---
name: weekly-shopping-list
description: >-
  Fetch the weekly meal plan from Mealie, combine duplicate ingredients by
  adding quantities, and sync the consolidated shopping list to Apple Reminders.
  Use when the user wants to generate a shopping list, sync ingredients to
  Reminders, or prepare for grocery shopping.
---

# Weekly Shopping List

Pull the current week's meal plan ingredients, merge duplicates, and push
each item to the Apple Reminders "Shopping" list.

## Workflow

### 1. Fetch raw ingredients

```bash
direnv exec . go run ./cmd/mealplan-ingredients
```

This prints each recipe's ingredients and an uncombined shopping list.

### 2. Combine duplicate ingredients

Parse the per-recipe ingredient blocks (not the uncombined list at the bottom).
Group by normalized food name, then merge quantities.

**Merging rules:**

| Scenario | Action | Example |
|---|---|---|
| Same food, same unit | Add quantities | 1 tsp + 2 tsp = 1 tbsp |
| Same food, compatible units | Convert then add | ½ tsp + 1 tsp = 1½ tsp |
| Same food, incompatible units | Keep separate lines | 1 can coconut milk + 3 cups coconut milk |
| Same food, no quantity | Deduplicate, keep one line | "Fresh parsley" x2 → "¼ cup fresh parsley" (use the one with a quantity) |
| Fractional avocados/similar | Round up to whole | 3/10 + 3/10 = 1 avocado |
| Water | Skip entirely | Not a shopping item |

**Unit conversions to apply when combining:**

- 3 tsp = 1 tbsp
- 16 tbsp = 1 cup
- 8 pinches ≈ 1 tsp (a pinch is ~⅛ tsp)

**Product distinctions to preserve:**

- Carton coconut milk (unsweetened, for drinking/smoothies) vs canned full-fat coconut milk (for cooking) — these are different products, don't merge.
- Ground cinnamon vs cinnamon sticks.
- Fresh vs dried herbs.

### 3. Read existing Shopping reminders

```
reminders_tasks action=read filterList=Shopping
```

Check what's already on the list to avoid creating duplicates.

### 4. Create reminders for each item

For each combined ingredient not already on the list:

```
reminders_tasks action=create title="<qty> <item>" targetList=Shopping
```

Format titles as `<quantity> <unit> <food name>` (e.g., "4 cups baby spinach").
For items without quantities, just use the food name (e.g., "Quinoa").

Batch the create calls in parallel groups of ~10 for speed.

## Notes

- The `mealplan-ingredients` tool requires `MEALIE_BASE` and `MEALIE_TOKEN` env vars, which direnv loads from `.envrc`.
- The Apple Reminders MCP server is `project-0-recipes-apple-reminders` with tool `reminders_tasks`.
- The target list name is "Shopping" (case-sensitive).
