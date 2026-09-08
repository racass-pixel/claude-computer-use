// Package recipes stores and replays parameterised multi-step desktop procedures.
package recipes

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// batchableTools is the set of tools allowed in recipe steps.
var batchableTools = map[string]bool{
	"click": true, "move": true, "drag": true, "scroll": true,
	"type": true, "key": true, "wait": true, "window": true,
	"clipboard": true, "find": true,
}

// Step is one action in a recipe.
type Step struct {
	Tool string         `json:"tool"`
	Args map[string]any `json:"args,omitempty"`
	Note string         `json:"note,omitempty"`
}

// Recipe is a saved, parameterised sequence of desktop actions.
type Recipe struct {
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	App         string    `json:"app,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Params      []string  `json:"params,omitempty"`
	Steps       []Step    `json:"steps"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Runs        int       `json:"runs"`
	Successes   int       `json:"successes"`
	Auto        bool      `json:"auto,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
}

// Match is a search result.
type Match struct {
	Recipe Recipe  `json:"recipe"`
	Score  float64 `json:"score"`
}

// Store manages recipes on disk.
type Store struct {
	Dir string
}

// Open creates the store directory if needed.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{Dir: dir}, nil
}

// cyrillic maps lower-case Cyrillic runes to Latin transliterations.
var cyrillic = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d",
	'е': "e", 'ё': "yo", 'ж': "zh", 'з': "z", 'и': "i",
	'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n",
	'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t",
	'у': "u", 'ф': "f", 'х': "kh", 'ц': "ts", 'ч': "ch",
	'ш': "sh", 'щ': "shch", 'э': "e", 'ю': "yu", 'я': "ya",
	'ь': "", 'ъ': "",
}

var slugRe = regexp.MustCompile(`[a-z0-9]+`)

// Slugify produces a lower-case ASCII slug from a name.
func Slugify(name string) string {
	// Transliterate Cyrillic
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if lat, ok := cyrillic[r]; ok {
			b.WriteString(lat)
		} else {
			b.WriteRune(r)
		}
	}
	parts := slugRe.FindAllString(b.String(), -1)
	slug := strings.Join(parts, "-")
	if len(slug) > 60 {
		slug = slug[:60]
		// Trim trailing dash
		slug = strings.TrimRight(slug, "-")
	}
	if slug == "" {
		h := sha1.Sum([]byte(name))
		slug = fmt.Sprintf("recipe-%x", h[:4])
	}
	return slug
}

func validSlug(slug string) bool {
	if slug == "" {
		return false
	}
	for _, r := range slug {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
}

func (s *Store) path(slug string) (string, error) {
	if !validSlug(slug) {
		return "", fmt.Errorf("invalid slug %q", slug)
	}
	return filepath.Join(s.Dir, slug+".json"), nil
}

// Save writes a recipe to disk. If a recipe with the same slug exists, it replaces
// it and resets run counters. Auto-save recipes never overwrite curated (non-auto) ones;
// they get a -2, -3 … suffix instead.
func (s *Store) Save(r Recipe) (Recipe, error) {
	if len(r.Steps) == 0 {
		return Recipe{}, fmt.Errorf("recipe must have at least one step")
	}
	for i, st := range r.Steps {
		if !batchableTools[st.Tool] {
			return Recipe{}, fmt.Errorf("step %d: tool %q is not batchable", i, st.Tool)
		}
	}
	r.Slug = Slugify(r.Name)
	now := time.Now()

	// Auto recipes must not overwrite curated ones — find a free slug.
	if r.Auto {
		base := r.Slug
		for n := 2; ; n++ {
			p, err := s.path(r.Slug)
			if err != nil {
				return Recipe{}, err
			}
			existing, eerr := s.readFile(p)
			if eerr != nil {
				break // slot is free
			}
			if existing.Auto {
				break // ok to overwrite another auto recipe
			}
			r.Slug = fmt.Sprintf("%s-%d", base, n)
		}
	}

	p, err := s.path(r.Slug)
	if err != nil {
		return Recipe{}, err
	}
	if existing, eerr := s.readFile(p); eerr == nil {
		r.CreatedAt = existing.CreatedAt
		// A manual save on the same slug resets counters (recipe was updated/replaced).
		if !r.Auto {
			r.Runs = 0
			r.Successes = 0
			r.LastError = ""
		} else {
			r.Runs = existing.Runs
			r.Successes = existing.Successes
		}
	} else {
		r.CreatedAt = now
	}
	r.UpdatedAt = now

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return Recipe{}, err
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return Recipe{}, err
	}
	return r, nil
}

func (s *Store) readFile(p string) (Recipe, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return Recipe{}, err
	}
	var r Recipe
	if err := json.Unmarshal(data, &r); err != nil {
		return Recipe{}, err
	}
	return r, nil
}

// Get returns a recipe by slug.
func (s *Store) Get(slug string) (Recipe, error) {
	p, err := s.path(slug)
	if err != nil {
		return Recipe{}, err
	}
	return s.readFile(p)
}

// List returns all recipes.
func (s *Store) List() ([]Recipe, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, err
	}
	var out []Recipe
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		r, err := s.readFile(filepath.Join(s.Dir, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// Delete removes a recipe.
func (s *Store) Delete(slug string) error {
	p, err := s.path(slug)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

// tokenize splits text into lower-case tokens of 2+ runes.
func tokenize(text string) []string {
	words := splitOnNonAlphaNum(strings.ToLower(text))
	var out []string
	for _, w := range words {
		if runeLen(w) >= 2 {
			out = append(out, w)
		}
	}
	return out
}

func splitOnNonAlphaNum(s string) []string {
	var parts []string
	var cur strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(r)
		} else {
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

// fuzzyMatchAny returns true if token fuzzy-matches any candidate.
// A fuzzy match requires a shared prefix of >= 4 runes that covers >= 50%
// of the shorter token. This handles morphological variation in inflected
// languages (e.g., Russian: "открыть" vs "открываю" share "откр" = 4/7 = 57%)
// while rejecting unrelated words that happen to start the same way
// (e.g., "настроить" vs "настолько" share "наст" = 4/9 = 44% < 50%).
func fuzzyMatchAny(token string, candidates []string) bool {
	tLen := utf8.RuneCountInString(token)
	if tLen < 4 {
		return false
	}
	for _, c := range candidates {
		cLen := utf8.RuneCountInString(c)
		shared := sharedPrefixRunes(token, c)
		shorter := tLen
		if cLen < shorter {
			shorter = cLen
		}
		if shared >= 4 && float64(shared) >= 0.5*float64(shorter) {
			return true
		}
	}
	return false
}

// sharedPrefixRunes returns the number of runes shared at the start of a and b.
func sharedPrefixRunes(a, b string) int {
	n := 0
	ra, rb := []rune(a), []rune(b)
	for i := 0; i < len(ra) && i < len(rb); i++ {
		if ra[i] != rb[i] {
			break
		}
		n++
	}
	return n
}

func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

// Search finds recipes matching the query.
func (s *Store) Search(query, app string, limit int) ([]Match, error) {
	if limit <= 0 {
		limit = 5
	}
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	qTokens := tokenize(query)
	if len(qTokens) == 0 {
		return nil, nil
	}
	qSet := make(map[string]bool, len(qTokens))
	for _, t := range qTokens {
		qSet[t] = true
	}

	var matches []Match
	for _, r := range all {
		// Build document tokens from name + description + tags
		docText := r.Name + " " + r.Description
		for _, tag := range r.Tags {
			docText += " " + tag
		}
		docTokens := tokenize(docText)
		docSet := make(map[string]bool, len(docTokens))
		for _, t := range docTokens {
			docSet[t] = true
		}

		// Score = |intersection| / |query tokens|
		// Uses fuzzy matching: exact match or shared prefix >= 4 runes (handles
		// morphological variation in inflected languages like Russian).
		hit := 0
		for _, qt := range qTokens {
			if docSet[qt] || fuzzyMatchAny(qt, docTokens) {
				hit++
			}
		}
		score := float64(hit) / float64(len(qTokens))

		// Name bonus: if every query token appears in the name
		nameTokens := tokenize(r.Name)
		nameSet := make(map[string]bool, len(nameTokens))
		for _, t := range nameTokens {
			nameSet[t] = true
		}
		allInName := true
		for _, qt := range qTokens {
			if !nameSet[qt] && !fuzzyMatchAny(qt, nameTokens) {
				allInName = false
				break
			}
		}
		if allInName {
			score += 0.15
		}

		// App bonus
		if app != "" && strings.EqualFold(r.App, app) {
			score += 0.3
		}

		// Curated (non-auto) bonus
		if !r.Auto {
			score += 0.1
		}

		// Success rate bonus
		if r.Runs > 0 {
			score += 0.1 * float64(r.Successes) / float64(r.Runs)
		}

		// Drop recipes that have been run ≥ 2 times with zero successes
		if r.Runs >= 2 && r.Successes == 0 {
			continue
		}

		if score < 0.25 {
			continue
		}
		matches = append(matches, Match{Recipe: r, Score: score})
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Score > matches[j].Score })
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

// Render substitutes {{param}} placeholders in recipe steps.
func Render(r Recipe, params map[string]string) ([]Step, error) {
	// Validate all declared params are provided
	for _, p := range r.Params {
		if _, ok := params[p]; !ok {
			return nil, fmt.Errorf("missing param %q", p)
		}
	}

	out := make([]Step, len(r.Steps))
	for i, st := range r.Steps {
		out[i] = Step{Tool: st.Tool, Note: st.Note}
		if len(st.Args) > 0 {
			out[i].Args = make(map[string]any, len(st.Args))
			for k, v := range st.Args {
				if sv, ok := v.(string); ok {
					for pk, pv := range params {
						sv = strings.ReplaceAll(sv, "{{"+pk+"}}", pv)
					}
					out[i].Args[k] = sv
				} else {
					out[i].Args[k] = v
				}
			}
		}
	}
	return out, nil
}

// Bump increments the run/success counters; on failure stores the error text.
func (s *Store) Bump(slug string, ok bool, lastErr string) error {
	p, err := s.path(slug)
	if err != nil {
		return err
	}
	r, err := s.readFile(p)
	if err != nil {
		return err
	}
	r.Runs++
	if ok {
		r.Successes++
		r.LastError = ""
	} else {
		r.LastError = lastErr
	}
	r.UpdatedAt = time.Now()
	data, merr := json.MarshalIndent(r, "", "  ")
	if merr != nil {
		return merr
	}
	return os.WriteFile(p, data, 0o644)
}

// TraceEntry is a completed tool invocation recorded by the server.
type TraceEntry struct {
	Tool    string
	Args    map[string]any
	Summary string
	OK      bool
}

// actionTools are the tools that represent real user actions (for draft building).
var actionTools = map[string]bool{
	"click": true, "drag": true, "scroll": true,
	"type": true, "key": true, "window": true,
	"wait": true, "click_until": true, "find": true,
	"mouse_down": true, "mouse_up": true,
}

// Draft builds a Recipe from a trace of completed actions.
// It keeps only action tools, collapses consecutive waits, parameterises long type texts,
// inserts wait{stable:true} after window focus / key win+*, and sets Auto: true.
func Draft(trace []TraceEntry, name, description, app string) Recipe {
	var steps []Step
	var params []string
	paramN := 0

	for i, e := range trace {
		if !actionTools[e.Tool] {
			continue
		}
		args := cloneArgs(e.Args)

		// Strip screenshot/region/pixel fields from args
		delete(args, "screenshot")
		delete(args, "screenshot_region")

		// Collapse consecutive waits: skip if prev step is also wait
		if e.Tool == "wait" && len(steps) > 0 && steps[len(steps)-1].Tool == "wait" {
			continue
		}

		// Parameterise long type texts (> 3 words)
		if e.Tool == "type" {
			if text, ok := args["text"].(string); ok {
				if wordCount(text) > 3 {
					paramN++
					pname := fmt.Sprintf("text%d", paramN)
					params = append(params, pname)
					args["text"] = "{{" + pname + "}}"
				}
			}
		}

		steps = append(steps, Step{Tool: e.Tool, Args: args, Note: e.Summary})

		// Insert wait{stable:true} after window focus or key win+*
		needWait := false
		if e.Tool == "window" {
			if act, ok := e.Args["action"].(string); ok && act == "focus" {
				needWait = true
			}
		}
		if e.Tool == "key" {
			k, _ := e.Args["key"].(string)
			if strings.HasPrefix(strings.ToLower(k), "win+") {
				needWait = true
			}
			// Also check keys array
			if ks, ok := e.Args["keys"].([]any); ok {
				for _, kv := range ks {
					if s, ok := kv.(string); ok && strings.HasPrefix(strings.ToLower(s), "win+") {
						needWait = true
					}
				}
			}
		}
		if needWait {
			// Don't add if next trace entry is already a wait
			nextIsWait := i+1 < len(trace) && trace[i+1].Tool == "wait"
			if !nextIsWait {
				steps = append(steps, Step{Tool: "wait", Args: map[string]any{"stable": true}})
			}
		}
	}

	if description == "" && name != "" {
		description = name
		if app != "" {
			description += " (" + app + ")"
		}
	}

	return Recipe{
		Name:        name,
		Slug:        Slugify(name),
		Description: description,
		App:         app,
		Params:      params,
		Steps:       steps,
		Auto:        true,
	}
}

func cloneArgs(m map[string]any) map[string]any {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func wordCount(s string) int {
	n := 0
	inWord := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' {
			inWord = false
		} else if !inWord {
			inWord = true
			n++
		}
	}
	return n
}
