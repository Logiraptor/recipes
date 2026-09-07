// Command recipes-web serves the recipes/ and meal-plans/ directories as a
// small read-only website. The repo is the database: there is no other store.
package main

import (
	"embed"
	"errors"
	"flag"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/patrickoyarzun/recipes/internal/recipedata"
)

//go:embed templates/*.html
var templateFS embed.FS

var funcs = template.FuncMap{
	"duration": humanDuration,
	"weekday":  weekday,
	"prettyDate": func(date string) string {
		t, err := time.Parse(recipedata.DateLayout, date)
		if err != nil {
			return date
		}
		return t.Format("Mon, Jan 2")
	},
	"title":   strings.Title, //nolint:staticcheck // ASCII meal names only
	"add":     func(a, b int) int { return a + b },
	"today":   func() string { return time.Now().Format(recipedata.DateLayout) },
	"excerpt": excerpt,
}

// Each page defines its own "content" block, so pages get their own template
// set layered on the shared layout rather than one big set.
var templates = parsePages("index.html", "recipe.html", "plan.html", "error.html")

func parsePages(names ...string) map[string]*template.Template {
	pages := make(map[string]*template.Template, len(names))
	for _, name := range names {
		pages[name] = template.Must(template.New(name).Funcs(funcs).
			ParseFS(templateFS, "templates/layout.html", "templates/"+name))
	}
	return pages
}

func main() {
	addr := flag.String("addr", envOr("ADDR", ":8080"), "listen address")
	flag.Parse()

	store, err := recipedata.Open()
	if err != nil {
		slog.Error("locate data root", "err", err)
		os.Exit(1)
	}
	slog.Info("serving recipes", "root", store.Root, "addr", *addr)

	srv := &server{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", srv.index)
	mux.HandleFunc("GET /recipes/{slug}", srv.recipe)
	mux.HandleFunc("GET /plan", srv.plan)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok\n"))
	})

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           logging(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := httpSrv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type server struct {
	store *recipedata.Store
}

// recipeCard is an index-page recipe plus how many times it's been cooked.
type recipeCard struct {
	*recipedata.Recipe
	Made int
	// Minutes is the recipe's total time in minutes, 0 when unknown. It is
	// used for the "quickest first" ordering.
	Minutes int
}

// categoryCount drives the category filter chips on the index page.
type categoryCount struct {
	Name  string
	Count int
}

// sortOptions are the index-page orderings, in the order they're shown.
var sortOptions = []struct{ Key, Label string }{
	{"cooked", "Most cooked"},
	{"name", "A–Z"},
	{"time", "Quickest"},
}

func (s *server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		s.fail(w, r, http.StatusNotFound, errors.New("not found"))
		return
	}
	recipes, err := s.store.Recipes()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	order := r.URL.Query().Get("sort")

	// Category chips count the search results, not the filtered-by-category
	// ones, so switching categories never shows a dead end.
	if query != "" {
		recipes = filter(recipes, query)
	}
	categories := categoryCounts(recipes)
	if category != "" {
		recipes = byCategory(recipes, category)
	}

	counts, err := s.store.MadeCounts(time.Now())
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err)
		return
	}
	// Recipes arrive in name order, so every stable sort below keeps
	// alphabetical order as the tie-breaker.
	cards := make([]recipeCard, 0, len(recipes))
	for _, rec := range recipes {
		cards = append(cards, recipeCard{
			Recipe:  rec,
			Made:    counts[rec.Slug],
			Minutes: durationMinutes(rec.TotalTime),
		})
	}
	switch order {
	case "name":
	case "time":
		// Unknown times sort last: an unlabelled recipe isn't a quick one.
		sort.SliceStable(cards, func(i, j int) bool {
			a, b := cards[i].Minutes, cards[j].Minutes
			if (a == 0) != (b == 0) {
				return b == 0
			}
			return a < b
		})
	default:
		order = "cooked"
		sort.SliceStable(cards, func(i, j int) bool { return cards[i].Made > cards[j].Made })
	}

	s.render(w, r, "index.html", map[string]any{
		"Title":       "Recipes",
		"Recipes":     cards,
		"Query":       query,
		"Category":    category,
		"Categories":  categories,
		"Sort":        order,
		"SortOptions": sortOptions,
	})
}

func (s *server) recipe(w http.ResponseWriter, r *http.Request) {
	recipe, err := s.store.Recipe(r.PathValue("slug"))
	if errors.Is(err, fs.ErrNotExist) {
		s.fail(w, r, http.StatusNotFound, err)
		return
	}
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err)
		return
	}
	s.render(w, r, "recipe.html", map[string]any{
		"Title":  recipe.Name,
		"Recipe": recipe,
	})
}

func (s *server) plan(w http.ResponseWriter, r *http.Request) {
	week := r.URL.Query().Get("week")
	if week == "" {
		week = recipedata.WeekStart(time.Now())
	}
	weeks, err := s.store.Weeks()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err)
		return
	}
	plan, err := s.store.MealPlan(week)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		s.fail(w, r, http.StatusInternalServerError, err)
		return
	}

	type slot struct {
		recipedata.Entry
		Recipe *recipedata.Recipe
	}
	var slots []slot
	if plan != nil {
		for _, e := range plan.Entries {
			cur := slot{Entry: e}
			if e.Recipe != "" {
				if rec, err := s.store.Recipe(e.Recipe); err == nil {
					cur.Recipe = rec
				}
			}
			slots = append(slots, cur)
		}
	}

	// Weeks is newest-first, so the previous week is the next index.
	var prev, next string
	for i, wk := range weeks {
		if wk != week {
			continue
		}
		if i+1 < len(weeks) {
			prev = weeks[i+1]
		}
		if i > 0 {
			next = weeks[i-1]
		}
	}

	s.render(w, r, "plan.html", map[string]any{
		"Title": "Meal plan " + week,
		"Week":  week,
		"Weeks": weeks,
		"Prev":  prev,
		"Next":  next,
		"Plan":  plan,
		"Slots": slots,
	})
}

func (s *server) render(w http.ResponseWriter, r *http.Request, name string, data map[string]any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page, ok := templates[name]
	if !ok {
		slog.Error("unknown template", "template", name)
		return
	}
	if err := page.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("render", "template", name, "path", r.URL.Path, "err", err)
	}
}

func (s *server) fail(w http.ResponseWriter, r *http.Request, code int, err error) {
	slog.Warn("request failed", "path", r.URL.Path, "status", code, "err", err)
	w.WriteHeader(code)
	s.render(w, r, "error.html", map[string]any{
		"Title":  http.StatusText(code),
		"Status": code,
	})
}

func filter(recipes []*recipedata.Recipe, query string) []*recipedata.Recipe {
	needle := strings.ToLower(query)
	var out []*recipedata.Recipe
	for _, r := range recipes {
		haystack := strings.ToLower(strings.Join(append([]string{
			r.Name, r.Description, r.RecipeCategory, r.RecipeCuisine,
		}, r.RecipeIngredient...), "\n"))
		if strings.Contains(haystack, needle) {
			out = append(out, r)
		}
	}
	return out
}

// categoryCounts summarises recipeCategory across recipes, most common first.
func categoryCounts(recipes []*recipedata.Recipe) []categoryCount {
	counts := make(map[string]int)
	for _, r := range recipes {
		if r.RecipeCategory != "" {
			counts[r.RecipeCategory]++
		}
	}
	out := make([]categoryCount, 0, len(counts))
	for name, n := range counts {
		out = append(out, categoryCount{Name: name, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func byCategory(recipes []*recipedata.Recipe, category string) []*recipedata.Recipe {
	var out []*recipedata.Recipe
	for _, r := range recipes {
		if strings.EqualFold(r.RecipeCategory, category) {
			out = append(out, r)
		}
	}
	return out
}

var isoDuration = regexp.MustCompile(`^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?)?$`)

// humanDuration turns an ISO-8601 duration like PT1H30M into "1 hr 30 min".
func humanDuration(d string) string {
	m := isoDuration.FindStringSubmatch(d)
	if m == nil {
		return d
	}
	var parts []string
	for i, unit := range []string{"day", "hr", "min"} {
		n, _ := strconv.Atoi(m[i+1])
		if n == 0 {
			continue
		}
		label := unit
		if unit == "day" && n > 1 {
			label = "days"
		}
		parts = append(parts, strconv.Itoa(n)+" "+label)
	}
	if len(parts) == 0 {
		return d
	}
	return strings.Join(parts, " ")
}

// durationMinutes converts an ISO-8601 duration to minutes, 0 if unparseable.
func durationMinutes(d string) int {
	m := isoDuration.FindStringSubmatch(d)
	if m == nil {
		return 0
	}
	days, _ := strconv.Atoi(m[1])
	hours, _ := strconv.Atoi(m[2])
	mins, _ := strconv.Atoi(m[3])
	return days*24*60 + hours*60 + mins
}

// excerpt shortens text to at most n characters on a word boundary, so card
// summaries stay one predictable size instead of relying on CSS clamping.
func excerpt(n int, text string) string {
	if len(text) <= n {
		return text
	}
	cut := text[:n]
	if i := strings.LastIndexAny(cut, " \t\n"); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,.;:—-") + "…"
}

func weekday(date string) string {
	t, err := time.Parse(recipedata.DateLayout, date)
	if err != nil {
		return date
	}
	return t.Format("Monday")
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start))
	})
}
