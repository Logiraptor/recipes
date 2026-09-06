// Schema for files in meal-plans/.
//
// One JSON file per week, named by the week's start date (Sunday):
//   meal-plans/2025-06-01.json
//
// Entries reference recipes by slug — the basename of a file in recipes/.
package schema

// Calendar date, YYYY-MM-DD.
#Date: =~"^\\d{4}-\\d{2}-\\d{2}$"

#Meal: "breakfast" | "lunch" | "dinner" | "snack" | "side"

#MealPlan: {
	// Sunday of the week this plan covers. Must equal the filename.
	weekStart!: #Date

	entries!: [...#Entry]

	notes?: string & !=""
}

// Exactly one of recipe / title is set: a slug pointing at recipes/<slug>.json,
// or a free-text title for things that aren't real recipes ("Leftovers",
// "Takeout").
#Entry: #EntryBase & ({
	recipe: #Slug
	title?: _|_
} | {
	title:   string & !=""
	recipe?: _|_
})

#EntryBase: {
	date!: #Date
	meal!: #Meal

	// Override the recipe's yield for this occasion.
	servings?: int & >0

	notes?: string & !=""
	...
}
