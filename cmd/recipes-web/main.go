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
	"title": strings.Title, //nolint:staticcheck // ASCII meal names only
	"add":   func(a, b int) int { return a + b },
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
	if query != "" {
		recipes = filter(recipes, query)
	}
	counts, err := s.store.MadeCounts(time.Now())
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err)
		return
	}
	// Most-cooked first; recipes come in name order, so the stable sort keeps
	// alphabetical order within a count.
	cards := make([]recipeCard, 0, len(recipes))
	for _, rec := range recipes {
		cards = append(cards, recipeCard{Recipe: rec, Made: counts[rec.Slug]})
	}
	sort.SliceStable(cards, func(i, j int) bool { return cards[i].Made > cards[j].Made })

	s.render(w, r, "index.html", map[string]any{
		"Title":   "Recipes",
		"Recipes": cards,
		"Query":   query,
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

	s.render(w, r, "plan.html", map[string]any{
		"Title": "Meal plan " + week,
		"Week":  week,
		"Weeks": weeks,
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
