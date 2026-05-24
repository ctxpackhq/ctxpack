package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
)

const (
	tokenBudget   = 8000
	charsPerToken = 4
	charBudget    = tokenBudget * charsPerToken
)

var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"vendor":       true,
	".svn":         true,
	".hg":          true,
	"dist":         true,
	"build":        true,
	".next":        true,
	"__pycache__":  true,
	".mypy_cache":  true,
	".pytest_cache": true,
	"coverage":     true,
}

var codeExts = map[string]bool{
	".go": true, ".py": true, ".js": true, ".ts": true, ".jsx": true,
	".tsx": true, ".java": true, ".c": true, ".cpp": true, ".cc": true,
	".h": true, ".hpp": true, ".cs": true, ".rb": true, ".rs": true,
	".swift": true, ".kt": true, ".scala": true, ".php": true, ".sh": true,
	".bash": true, ".zsh": true, ".fish": true, ".lua": true, ".r": true,
	".ex": true, ".exs": true, ".erl": true, ".hs": true, ".ml": true,
	".clj": true, ".cljs": true, ".dart": true, ".vim": true, ".tf": true,
	".hcl": true, ".yaml": true, ".yml": true, ".toml": true, ".json": true,
	".xml": true, ".html": true, ".css": true, ".scss": true, ".sass": true,
	".less": true, ".sql": true, ".md": true, ".rst": true, ".txt": true,
	".env": true, ".dockerfile": true, ".makefile": true, ".mk": true,
	".mod": true, ".sum": true, ".lock": true, ".proto": true, ".graphql": true,
}

var skipFilenames = map[string]bool{
	"package-lock.json": true,
	"yarn.lock":         true,
	"pnpm-lock.yaml":    true,
	"go.sum":            true,
	"Gemfile.lock":      true,
	"poetry.lock":       true,
	"composer.lock":     true,
	"Cargo.lock":        true,
}

type fileEntry struct {
	path    string
	content string
	score   float64
	tokens  int
}

func main() {
	var root string

	cmd := &cobra.Command{
		Use:   "ctxpack \"task description\"",
		Short: "Pack relevant code files as context for a given task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task := args[0]
			return run(task, root)
		},
	}

	cmd.Flags().StringVarP(&root, "dir", "d", ".", "Root directory to scan")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(task, root string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolving root: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Scanning %s...\n", absRoot)

	files, err := collectFiles(absRoot)
	if err != nil {
		return fmt.Errorf("collecting files: %w", err)
	}

	scanned := len(files)
	fmt.Fprintf(os.Stderr, "Scoring %d files...\n", scanned)

	scored := scoreFiles(files, task)

	selected := selectFiles(scored)

	output := formatOutput(task, selected)
	tokens := estimateTokens(output)

	if err := clipboard.WriteAll(output); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not copy to clipboard: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "Copied to clipboard.\n")
	}

	fmt.Fprintf(os.Stdout, "%s\n", output)

	fmt.Fprintf(os.Stderr, "\n--- Summary ---\n")
	fmt.Fprintf(os.Stderr, "Files scanned:  %d\n", scanned)
	fmt.Fprintf(os.Stderr, "Files selected: %d\n", len(selected))
	fmt.Fprintf(os.Stderr, "Token estimate: %d / %d\n", tokens, tokenBudget)

	return nil
}

func collectFiles(root string) ([]fileEntry, error) {
	var files []fileEntry

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}

		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			// skip hidden dirs (except root)
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		name := d.Name()
		if skipFilenames[strings.ToLower(name)] {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(name))
		baseLower := strings.ToLower(name)

		// allow files with no extension that are known names
		knownNoExt := map[string]bool{
			"makefile": true, "dockerfile": true, "rakefile": true,
			"gemfile": true, "procfile": true, "vagrantfile": true,
		}
		if ext == "" && !knownNoExt[baseLower] {
			return nil
		}
		if ext != "" && !codeExts[ext] {
			return nil
		}

		info, err := d.Info()
		if err != nil || info.Size() == 0 {
			return nil
		}
		// skip files larger than 200KB — likely generated or binary
		if info.Size() > 200*1024 {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// skip binary files
		if isBinary(data) {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		files = append(files, fileEntry{
			path:    rel,
			content: string(data),
			tokens:  estimateTokens(string(data)),
		})
		return nil
	})

	return files, err
}

func isBinary(data []byte) bool {
	check := data
	if len(check) > 512 {
		check = check[:512]
	}
	for _, b := range check {
		if b == 0 {
			return true
		}
		if b < 7 || (b > 14 && b < 32 && b != 27) {
			// control chars other than tab/newline/cr/etc
			nonPrint := 0
			for _, c := range check {
				if c < 9 || (c > 13 && c < 32) {
					nonPrint++
				}
			}
			return nonPrint > len(check)/10
		}
	}
	return false
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var cur strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			cur.WriteRune(r)
		} else {
			if cur.Len() > 1 {
				tokens = append(tokens, cur.String())
			}
			cur.Reset()
		}
	}
	if cur.Len() > 1 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

func termFreq(tokens []string) map[string]float64 {
	tf := make(map[string]float64)
	for _, t := range tokens {
		tf[t]++
	}
	n := float64(len(tokens))
	if n == 0 {
		return tf
	}
	for k := range tf {
		tf[k] /= n
	}
	return tf
}

func scoreFiles(files []fileEntry, task string) []fileEntry {
	queryTokens := tokenize(task)
	queryTF := termFreq(queryTokens)

	// build IDF: count how many documents contain each term
	docFreq := make(map[string]int)
	tokenSets := make([]map[string]float64, len(files))
	for i, f := range files {
		toks := tokenize(f.content)
		tf := termFreq(toks)
		tokenSets[i] = tf
		seen := make(map[string]bool)
		for t := range tf {
			if !seen[t] {
				docFreq[t]++
				seen[t] = true
			}
		}
	}

	N := float64(len(files))

	for i := range files {
		var score float64
		for term, qw := range queryTF {
			df := docFreq[term]
			if df == 0 {
				continue
			}
			idf := math.Log(N/float64(df)) + 1
			docTF := tokenSets[i][term]
			score += qw * docTF * idf
		}

		// boost: path contains query terms
		pathLower := strings.ToLower(files[i].path)
		for term := range queryTF {
			if strings.Contains(pathLower, term) {
				score += 0.1
			}
		}

		if strings.HasSuffix(pathLower, "_test.go") {
			score *= 0.5
		}

		files[i].score = score
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].score > files[j].score
	})

	return files
}

func selectFiles(files []fileEntry) []fileEntry {
	var selected []fileEntry
	budget := charBudget

	for _, f := range files {
		if f.score == 0 {
			break
		}
		cost := len(f.content)
		if cost > budget {
			// try to fit a truncated version if the file is large
			continue
		}
		budget -= cost
		selected = append(selected, f)
		if budget <= 0 {
			break
		}
	}

	return selected
}

func formatOutput(task string, files []fileEntry) string {
	var sb strings.Builder

	sb.WriteString("# Context for: ")
	sb.WriteString(task)
	sb.WriteString("\n\n")

	for _, f := range files {
		sb.WriteString("## ")
		sb.WriteString(f.path)
		sb.WriteString("\n\n```\n")
		sb.WriteString(f.content)
		if !strings.HasSuffix(f.content, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("```\n\n")
	}

	return strings.TrimRight(sb.String(), "\n") + "\n"
}

func estimateTokens(s string) int {
	return (len(s) + charsPerToken - 1) / charsPerToken
}
