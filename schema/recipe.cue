// Schema for files in recipes/.
//
// One JSON file per recipe. The file's basename (without .json) is the
// recipe's slug, and is what meal-plans/ files reference.
//
// The shape is a pragmatic subset of schema.org/Recipe: string durations are
// ISO-8601 so the files stay valid schema.org JSON if we ever publish them.
package schema

// ISO-8601 duration, e.g. "PT30M", "PT1H15M".
#Duration: =~"^P(T(\\d+H)?(\\d+M)?(\\d+S)?)?$"

// Lowercase kebab-case identifier, matching the filename.
#Slug: =~"^[a-z0-9]+(-[a-z0-9]+)*$"

#Recipe: {
	// Human-readable title.
	name!: string & !=""

	// One or two sentences describing the dish.
	description?: string & !=""

	// Free-form list, one ingredient per line, quantity first.
	// e.g. "2 tablespoons olive oil"
	recipeIngredient!: [...string & !=""] & [_, ...]

	// Ordered steps, one per entry.
	recipeInstructions!: [...string & !=""] & [_, ...]

	// e.g. "4 servings"
	recipeYield?: string & !=""

	recipeCategory?: #Category
	recipeCuisine?:  string & !=""

	prepTime?:  #Duration
	cookTime?:  #Duration
	totalTime?: #Duration

	// Equipment needed, e.g. "Instant Pot".
	tool?: [...string & !=""]

	// Search tags, e.g. "weeknight", "freezer-friendly".
	keywords?: [...string & !=""]

	suitableForDiet?: [...#Diet]

	nutrition?: #Nutrition

	// Provenance: where the recipe came from.
	source?: string & !=""

	// Free-form notes not part of the instructions.
	notes?: string & !=""
}

#Category: "Breakfast" | "Lunch" | "Dinner" | "Side" | "Salad" | "Soup" |
	"Snack" | "Dessert" | "Bread" | "Drink" | "Sauce"

#Diet: "DiabeticDiet" | "GlutenFreeDiet" | "HalalDiet" | "HinduDiet" |
	"KosherDiet" | "LowCalorieDiet" | "LowFatDiet" | "LowLactoseDiet" |
	"LowSaltDiet" | "VeganDiet" | "VegetarianDiet"

#Nutrition: {
	calories?:      string
	proteinContent?: string
	fatContent?:     string
	carbohydrateContent?: string
	fiberContent?:   string
	sodiumContent?:  string
	sugarContent?:   string
}
