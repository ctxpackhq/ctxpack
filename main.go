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
	gitignore "github.com/sabhiram/go-gitignore"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	defaultTokenBudget = 8000
	charsPerToken      = 4
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
	".dockerfile": true, ".makefile": true, ".mk": true,
	".mod": true, ".sum": true, ".lock": true, ".proto": true, ".graphql": true,
}

var skipFilenames = map[string]bool{
	"package-lock.json": true,
	"yarn.lock":         true,
	"pnpm-lock.yaml":    true,
	"go.sum":            true,
	"gemfile.lock":      true,
	"poetry.lock":       true,
	"composer.lock":     true,
	"cargo.lock":        true,
	".env":              true,
	".env.local":        true,
}

var knownNoExt = map[string]bool{
	"makefile": true, "dockerfile": true, "rakefile": true,
	"gemfile": true, "procfile": true, "vagrantfile": true,
}

type fileEntry struct {
	path    string
	content string
	score   float64
	tokens  int
}

func main() {
	var root string
	var noClipboard bool
	var maxTokens int
	var preview bool

	cmd := &cobra.Command{
		Use:   "ctxpack \"task description\"",
		Short: "Pack relevant code files as context for a given task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task := args[0]
			return run(task, root, noClipboard, preview, maxTokens)
		},
	}

	cmd.Flags().StringVarP(&root, "dir", "d", ".", "Root directory to scan")
	cmd.Flags().BoolVar(&noClipboard, "no-clipboard", false, "Disable clipboard copy")
	cmd.Flags().BoolVar(&preview, "preview", false, "Show file scores and token counts without outputting content")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", defaultTokenBudget, "Token budget for selected files")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(task, root string, noClipboard, preview bool, maxTokens int) error {
	if maxTokens <= 0 {
		return fmt.Errorf("--max-tokens must be greater than zero")
	}
	if maxTokens > 1000000 {
		return fmt.Errorf("--max-tokens must not exceed 1000000")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolving root: %w", err)
	}
	dirInfo, statErr := os.Stat(absRoot)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return fmt.Errorf("directory does not exist: %s", absRoot)
		}
		return fmt.Errorf("accessing --dir path: %w", statErr)
	}
	if !dirInfo.IsDir() {
		return fmt.Errorf("not a directory: %s", absRoot)
	}

	fmt.Fprintf(os.Stderr, "Scanning %s...\n", absRoot)

	files, err := collectFiles(absRoot)
	if err != nil {
		return fmt.Errorf("collecting files: %w", err)
	}

	scanned := len(files)
	fmt.Fprintf(os.Stderr, "Scoring %d files...\n", scanned)

	scored := scoreFiles(files, task)

	selected := selectFiles(scored, maxTokens*charsPerToken)

	if preview {
		printPreview(selected)
		return nil
	}

	output := formatOutput(task, selected)
	var contentLen int
	for _, f := range selected {
		contentLen += len(f.content)
	}
	tokens := (contentLen + charsPerToken - 1) / charsPerToken

	if !noClipboard && term.IsTerminal(int(os.Stdout.Fd())) {
		if err := clipboard.WriteAll(output); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not copy to clipboard: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Copied to clipboard.\n")
		}
	}

	fmt.Fprint(os.Stdout, output)

	fmt.Fprintf(os.Stderr, "\n--- Summary ---\n")
	fmt.Fprintf(os.Stderr, "Files scanned:  %d\n", scanned)
	fmt.Fprintf(os.Stderr, "Files selected: %d\n", len(selected))
	fmt.Fprintf(os.Stderr, "Token estimate: %d / %d\n", tokens, maxTokens)

	return nil
}

func printPreview(files []fileEntry) {
	fileW := len("File")
	scoreW := len("Score")
	tokW := len("Tokens")

	type row struct {
		path  string
		score string
		tok   string
	}
	rows := make([]row, len(files))
	for i, f := range files {
		r := row{
			path:  f.path,
			score: fmt.Sprintf("%.2f", f.score),
			tok:   fmt.Sprintf("%d", f.tokens),
		}
		if len(r.path) > fileW {
			fileW = len(r.path)
		}
		if len(r.score) > scoreW {
			scoreW = len(r.score)
		}
		if len(r.tok) > tokW {
			tokW = len(r.tok)
		}
		rows[i] = r
	}

	hr := func(l, m, r string) string {
		return l + strings.Repeat("─", fileW+2) + m +
			strings.Repeat("─", scoreW+2) + m +
			strings.Repeat("─", tokW+2) + r
	}

	fmt.Println(hr("┌", "┬", "┐"))
	fmt.Printf("│ %-*s │ %-*s │ %-*s │\n", fileW, "File", scoreW, "Score", tokW, "Tokens")
	fmt.Println(hr("├", "┼", "┤"))

	totalTok := 0
	for i, r := range rows {
		totalTok += files[i].tokens
		fmt.Printf("│ %-*s │ %-*s │ %*s │\n", fileW, r.path, scoreW, r.score, tokW, r.tok)
	}

	fmt.Println(hr("└", "┴", "┘"))
	fmt.Printf("Total: %d files, %d tok\n", len(files), totalTok)
}

func collectFiles(root string) ([]fileEntry, error) {
	var files []fileEntry

	gi, _ := gitignore.CompileIgnoreFile(filepath.Join(root, ".gitignore"))

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}

		rel, _ := filepath.Rel(root, path)
		if gi != nil && rel != "." && gi.MatchesPath(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
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
		baseLower := strings.ToLower(name)
		if skipFilenames[baseLower] {
			return nil
		}
		// skip .env.* variants (e.g. .env.production, .env.test)
		if strings.HasPrefix(baseLower, ".env.") {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(name))

		if ext == "" && !knownNoExt[baseLower] {
			return nil
		}
		if ext != "" && !codeExts[ext] {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		// skip symlinks — don't follow them outside the repo
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if info.Size() == 0 || info.Size() > 200*1024 {
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
	}
	nonPrint := 0
	for _, b := range check {
		if b < 9 || (b > 13 && b < 32) {
			nonPrint++
		}
	}
	return nonPrint > len(check)/10
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

func isTestFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, ".spec.ts") || strings.HasSuffix(base, ".test.ts") ||
		strings.HasSuffix(base, ".spec.js") || strings.HasSuffix(base, ".test.js") ||
		strings.HasSuffix(base, "_test.py") || strings.HasSuffix(base, ".snap") {
		return true
	}
	if strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py") {
		return true
	}
	for _, component := range strings.Split(filepath.ToSlash(path), "/") {
		if component == "__tests__" {
			return true
		}
	}
	return false
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
		for t := range tf {
			docFreq[t]++
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

		if isTestFile(files[i].path) {
			score *= 0.5
		}

		files[i].score = score
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].score > files[j].score
	})

	return files
}

func selectFiles(files []fileEntry, charBudget int) []fileEntry {
	var selected []fileEntry
	budget := charBudget

	for _, f := range files {
		if f.score == 0 {
			break
		}
		cost := len(f.content)
		if cost > budget {
			continue
		}
		selected = append(selected, f)
		budget -= cost
	}

	return selected
}

func backtickFence(content string) string {
	max := 2
	run := 0
	for _, c := range content {
		if c == '`' {
			run++
			if run > max {
				max = run
			}
		} else {
			run = 0
		}
	}
	return strings.Repeat("`", max+1)
}

func formatOutput(task string, files []fileEntry) string {
	var sb strings.Builder

	sb.WriteString("# Context for: ")
	sb.WriteString(task)
	sb.WriteString("\n\n")

	for _, f := range files {
		fence := backtickFence(f.content)
		sb.WriteString("## ")
		sb.WriteString(f.path)
		sb.WriteString("\n\n")
		sb.WriteString(fence)
		sb.WriteString("\n")
		sb.WriteString(f.content)
		if !strings.HasSuffix(f.content, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString(fence)
		sb.WriteString("\n\n")
	}

	return strings.TrimRight(sb.String(), "\n") + "\n"
}

func estimateTokens(s string) int {
	return (len(s) + charsPerToken - 1) / charsPerToken
}
