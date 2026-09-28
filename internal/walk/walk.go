// Package walk discovers input files for recursive searches.
package walk

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// Options controls file discovery.
type Options struct {
	Recursive bool
	NoIgnore  bool
	Includes  []string
	Excludes  []string
}

// Collect resolves paths to a deterministic list of regular files. Explicit
// files retain argument order; files below directories are sorted by WalkDir.
func Collect(paths []string, opts Options) ([]string, error) {
	if len(paths) == 0 {
		if !opts.Recursive {
			return nil, nil
		}
		paths = []string{"."}
	}

	filters, err := compileFilters(opts.Includes, opts.Excludes)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var files []string
	var errs []error
	for _, name := range paths {
		info, err := os.Stat(name)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		if !info.IsDir() {
			if info.Mode().IsRegular() && filters.match(filepath.Base(name), false) {
				files = appendUnique(files, seen, name)
			}
			continue
		}
		if !opts.Recursive {
			errs = append(errs, fmt.Errorf("%s: is a directory (use --recursive)", name))
			continue
		}

		found, err := collectDir(name, opts.NoIgnore, filters)
		if err != nil {
			errs = append(errs, err)
		}
		for _, file := range found {
			files = appendUnique(files, seen, file)
		}
	}
	return files, errors.Join(errs...)
}

func appendUnique(files []string, seen map[string]struct{}, name string) []string {
	clean := filepath.Clean(name)
	if _, ok := seen[clean]; ok {
		return files
	}
	seen[clean] = struct{}{}
	return append(files, clean)
}

type filters struct {
	includes []*regexp.Regexp
	excludes []*regexp.Regexp
}

func compileFilters(includes, excludes []string) (filters, error) {
	var result filters
	for _, pattern := range includes {
		re, err := compileUserGlob(pattern)
		if err != nil {
			return filters{}, fmt.Errorf("invalid --include glob %q: %w", pattern, err)
		}
		result.includes = append(result.includes, re)
	}
	for _, pattern := range excludes {
		re, err := compileUserGlob(pattern)
		if err != nil {
			return filters{}, fmt.Errorf("invalid --exclude glob %q: %w", pattern, err)
		}
		result.excludes = append(result.excludes, re)
	}
	return result, nil
}

func compileUserGlob(pattern string) (*regexp.Regexp, error) {
	pattern = filepath.ToSlash(pattern)
	if !strings.Contains(pattern, "/") {
		pattern = "**/" + pattern
	}
	return compileGlob(pattern)
}

func (f filters) match(name string, isDir bool) bool {
	name = filepath.ToSlash(name)
	if isDir {
		name += "/"
	}
	for _, re := range f.excludes {
		if re.MatchString(name) {
			return false
		}
	}
	if isDir || len(f.includes) == 0 {
		return true
	}
	for _, re := range f.includes {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}

type ignoreRule struct {
	base    string
	pattern *regexp.Regexp
	negated bool
	dirOnly bool
}

func collectDir(root string, noIgnore bool, filters filters) ([]string, error) {
	rulesByDir := make(map[string][]ignoreRule)
	var files []string
	var errs []error
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, walkErr))
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			rel = ""
		}

		parentRules := rulesByDir[filepath.Dir(name)]
		if name == root {
			parentRules = nil
		}
		if rel != "" {
			if entry.IsDir() && entry.Name() == ".git" {
				return filepath.SkipDir
			}
			if ignored(rel, entry.IsDir(), parentRules) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !filters.match(rel, entry.IsDir()) {
				if entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if entry.IsDir() {
			rules := append([]ignoreRule(nil), parentRules...)
			if !noIgnore {
				local, err := readIgnoreFile(filepath.Join(name, ".gitignore"), rel)
				if err != nil {
					errs = append(errs, err)
				} else {
					rules = append(rules, local...)
				}
			}
			rulesByDir[name] = rules
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			return nil
		}
		if info.Mode().IsRegular() {
			files = append(files, name)
		}
		return nil
	})
	if err != nil {
		errs = append(errs, err)
	}
	return files, errors.Join(errs...)
}

func readIgnoreFile(name, base string) ([]ignoreRule, error) {
	file, err := os.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	defer file.Close()

	var rules []ignoreRule
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		escapedLeading := strings.HasPrefix(line, `\#`) || strings.HasPrefix(line, `\!`)
		if escapedLeading {
			line = line[1:]
		}
		negated := !escapedLeading && strings.HasPrefix(line, "!")
		if negated {
			line = line[1:]
		}
		dirOnly := strings.HasSuffix(line, "/")
		line = strings.TrimSuffix(line, "/")
		anchored := strings.HasPrefix(line, "/")
		line = strings.TrimPrefix(line, "/")
		if line == "" {
			continue
		}
		if !anchored && !strings.Contains(line, "/") {
			line = "**/" + line
		}
		re, err := compileGlob(line)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid pattern %q: %w", name, scanner.Text(), err)
		}
		rules = append(rules, ignoreRule{base: base, pattern: re, negated: negated, dirOnly: dirOnly})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return rules, nil
}

func ignored(name string, isDir bool, rules []ignoreRule) bool {
	ignored := false
	for _, rule := range rules {
		rel := name
		if rule.base != "" {
			prefix := strings.TrimSuffix(rule.base, "/") + "/"
			if !strings.HasPrefix(rel, prefix) {
				continue
			}
			rel = strings.TrimPrefix(rel, prefix)
		}
		if rule.dirOnly && !isDir {
			continue
		}
		if rule.pattern.MatchString(rel) {
			ignored = !rule.negated
		}
	}
	return ignored
}

// compileGlob implements the useful gitignore/include subset, including **.
// A single star never crosses a slash; ** does.
func compileGlob(pattern string) (*regexp.Regexp, error) {
	pattern = path.Clean(strings.TrimSpace(filepath.ToSlash(pattern)))
	if pattern == "." {
		return nil, errors.New("empty pattern")
	}
	var b strings.Builder
	b.WriteByte('^')
	for i := 0; i < len(pattern); {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i += 2
				if i < len(pattern) && pattern[i] == '/' {
					b.WriteString("(?:.*/)?")
					i++
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
				i++
			}
		case '?':
			b.WriteString("[^/]")
			i++
		case '[':
			end := strings.IndexByte(pattern[i+1:], ']')
			if end < 0 {
				return nil, errors.New("unterminated character class")
			}
			end += i + 1
			class := pattern[i+1 : end]
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			b.WriteByte('[')
			b.WriteString(class)
			b.WriteByte(']')
			i = end + 1
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
			i++
		}
	}
	b.WriteString("/?$")
	return regexp.Compile(b.String())
}
