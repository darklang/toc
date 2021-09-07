package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
	"gopkg.in/yaml.v3"
)

// -----------------------------
// Misc
// -----------------------------

func check(reason string, e error) {
	if e != nil {
		fmt.Printf("An unknown error occurred while %v\n", reason)
		panic(e)
	}
}

// -----------------------------
// Markdown escaping
// Copied directly from https://github.com/JohannesKaufmann/html-to-markdown/blob/8363feb9dd1247f1f6dd022f207fed45547f4e7f/escape/escape.go
// MIT license - Copyright (c) 2018 Johannes Kaufmann
// -----------------------------

var backslash = regexp.MustCompile(`\\(\S)`)
var heading = regexp.MustCompile(`(?m)^(#{1,6} )`)
var orderedList = regexp.MustCompile(`(?m)^(\W* {0,3})(\d+)\. `)
var unorderedList = regexp.MustCompile(`(?m)^([^\\\w]*)[*+-] `)
var horizontalDivider = regexp.MustCompile(`(?m)^([-*_] *){3,}$`)
var blockquote = regexp.MustCompile(`(?m)^(\W* {0,3})> `)

var replacer = strings.NewReplacer(
	`*`, `\*`,
	`_`, `\_`,
	"`", "\\`",
	`|`, `\|`,
)

// MarkdownCharacters escapes common markdown characters so that
// `<p>**Not Bold**</p> ends up as correct markdown `\*\*Not Strong\*\*`.
// No worry, the escaped characters will display fine, just without the formatting.
func MarkdownCharacters(text string) string {
	// Escape backslash escapes!
	text = backslash.ReplaceAllString(text, `\\$1`)

	// Escape headings
	text = heading.ReplaceAllString(text, `\$1`)

	// Escape hr
	text = horizontalDivider.ReplaceAllStringFunc(text, func(t string) string {
		if strings.Contains(t, "-") {
			return strings.Replace(t, "-", `\-`, 3)
		} else if strings.Contains(t, "_") {
			return strings.Replace(t, "_", `\_`, 3)
		}
		return strings.Replace(t, "*", `\*`, 3)
	})

	// Escape ol bullet points
	text = orderedList.ReplaceAllString(text, `$1$2\. `)

	// Escape ul bullet points
	text = unorderedList.ReplaceAllStringFunc(text, func(t string) string {
		return regexp.MustCompile(`([*+-])`).ReplaceAllString(t, `\$1`)
	})

	// Escape blockquote indents
	text = blockquote.ReplaceAllString(text, `$1\> `)

	// Escape em/strong *
	// Escape em/strong _
	// Escape code _
	text = replacer.Replace(text)

	// Escape link brackets
	// 	(disabled)
	// var link = regexp.MustCompile(`[\[\]]`)
	// text = link.ReplaceAllString(text, `\$&`)

	return text
}

// -----------------------------
// Ignores - appropriately ignore files
// -----------------------------

// We keep a list of matchers and test files in each
type GitIgnores struct {
	matchers []*ignore.GitIgnore
}

func (gi *GitIgnores) addFile(path string) {
	matchers, err := ignore.CompileIgnoreFile(path)
	check("adding gitignore file from "+path, err)
	gi.matchers = append(gi.matchers, matchers)
}

func (gi *GitIgnores) addList(paths []string) {
	matchers := ignore.CompileIgnoreLines(paths...)
	gi.matchers = append(gi.matchers, matchers)
}

func (gi *GitIgnores) addBuiltin(root string) {
	gi.addList([]string{".git", ".gitkeep", ".gitattributes", ".dockerignore", "node_modules", "README.md"})
	gi.addFile(root + "/.gitignore")
}

func (gi *GitIgnores) Match(path string) bool {
	for _, matcher := range gi.matchers {
		if matcher.MatchesPath(path) {
			return true
		}
	}
	return false
}

// -----------------------------
// Comment types
// -----------------------------

type Syntax interface {
	read(firstFiveLines []string) string
}

// -----------------------------
// Single line comments
// -----------------------------

type SingleLineComment struct {
	prefix string
}

func (c SingleLineComment) read(lines []string) string {
	started := false
	result := []string{}
	for _, line := range lines {
		if started {
			if strings.HasPrefix(line, c.prefix) {
				line = strings.TrimPrefix(line, c.prefix)
				result = append(result, line)
			} else {
				break
			}
		} else {
			if strings.HasPrefix(line, c.prefix) {
				started = true
				line = strings.TrimPrefix(line, c.prefix)
				result = append(result, strings.TrimSpace(line))
			}
		}
	}
	if len(result) == 0 {
		return ""
	} else {
		return strings.Join(result, "")
	}
}

// -----------------------------
// Multiline comments
// -----------------------------

type MultiLineComment struct {
	start string
	end   string
}

func (c MultiLineComment) read(lines []string) string {
	started := false
	result := []string{}
	for _, line := range lines {
		if started {
			if strings.HasSuffix(line, c.end) {
				line = strings.TrimSuffix(line, c.end)
				result = append(result, line)
				break
			} else {
				result = append(result, line)
			}
		} else {
			if strings.HasPrefix(line, c.start) {
				started = true
				line = strings.TrimPrefix(line, c.start)
				result = append(result, strings.TrimSpace(line))
			}
		}
	}
	if len(result) == 0 {
		return ""
	} else {
		return strings.Join(result, "")
	}
}

// -----------------------------
// Language definitions
// -----------------------------

type LanguageComment struct {
	language string
	syntaxes []Syntax
}

var defaultLanguageComment = LanguageComment{
	language: "Unknown",
	syntaxes: []Syntax{SingleLineComment{prefix: "//"}, SingleLineComment{prefix: "#"}},
}

var commentTable = map[string]LanguageComment{
	"go": {
		language: "Go",
		syntaxes: []Syntax{SingleLineComment{prefix: "//"}},
	},
	"md": {
		language: "Markdown",
		syntaxes: []Syntax{SingleLineComment{prefix: "#"}},
	},
	"fs": {
		language: "F#",
		syntaxes: []Syntax{SingleLineComment{prefix: "//"}},
	},
	"ml": {
		language: "OCaml",
		syntaxes: []Syntax{MultiLineComment{start: "(**", end: "*)"}, MultiLineComment{start: "(*", end: "*)"}},
	},
	"eot": {
		language: "Embedded OpenType",
		syntaxes: []Syntax{},
	},
}

func getFileDescription(path string) string {
	firstFewLines := make([]string, 0)

	// Open the file for scanning
	reader, err := os.Open(path)
	check("opening "+path, err)
	scanner := bufio.NewScanner(reader)

	// First line
	// hashBangLine := ""
	scanner.Scan()
	firstLine := scanner.Text()
	if strings.HasPrefix(firstLine, "#!") {
		// hashBangLine = firstLine
	} else {
		firstFewLines = append(firstFewLines, firstLine)
	}

	// Read another few lines
	for i := 0; i < 5; i++ {
		scanner.Scan()
		line := scanner.Text()
		firstFewLines = append(firstFewLines, line)
	}

	// Find the language
	extension := filepath.Ext(path)
	language, hasLanguage := commentTable[extension]
	if !hasLanguage {
		language = defaultLanguageComment
	}

	// Get the description from the syntax definitions
	description := ""
	for _, syntax := range language.syntaxes {
		description = syntax.read(firstFewLines)
		if description != "" {
			break
		}
	}
	return MarkdownCharacters(description)
}

// -----------------------------
// Records - store the data about each file
// -----------------------------

type Record struct {
	path        string
	description string
	isFile      bool
}

type Records = map[string]Record

func writeRecords(records Records, path string, description string, isFile bool) {
	records[path] = Record{path: path, description: description, isFile: isFile}
}

func collectRecords(dir string, cfg *Config, ignores *GitIgnores) Records {
	cfgNoListing := make(map[string]bool, len(cfg.Ignore))
	for _, dirName := range cfg.Directories.NoListing {
		cfgNoListing[dirName] = true
	}

	records := make(map[string]Record)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		check("Walking into "+path, err)

		// If we find more ignores as we go on, rebuild the matcher
		if strings.HasSuffix(path, "/.gitignore") {
			ignores.addFile(path)
			return nil
		}

		// Check if it's ignored via gitignore
		pathname := strings.TrimPrefix(path, dir)
		pathname = strings.TrimPrefix(pathname, "/")
		if ignores.Match(pathname) {
			fmt.Printf("ignoring: %q\n", pathname)
			if d.IsDir() {
				return filepath.SkipDir
			} else {
				return nil
			}
		}

		// Save description
		if d.IsDir() {
			desc := "a dir"
			value, hasCfgValue := cfg.Directories.Values[pathname]
			if hasCfgValue {
				fmt.Printf("using value for %q: %q\n", pathname, value)
				writeRecords(records, pathname, value, d.IsDir())
				return filepath.SkipDir
			} else {
				writeRecords(records, pathname, desc, d.IsDir())
				if cfgNoListing[pathname] {
					fmt.Printf("nolisting: %q\n", pathname)
					return filepath.SkipDir
				}
			}
		} else {
			desc := getFileDescription(path)
			writeRecords(records, pathname, desc, d.IsDir())
		}
		return nil
	})
	check("Walking the directory tree", err)
	return records
}

// -----------------------------
// Layout - convert file records a markdown string
// -----------------------------

// Recursive structure with all the results laid out, ready to print
type Layout struct {
	completePath string
	filename     string
	description  string
	children     map[string]Layout
}

func convertRecordsToLayout(records Records) Layout {
	paths := make([]string, 0, len(records))

	root := Layout{completePath: "", filename: "", description: "root", children: make(map[string]Layout)}

	for path := range records {
		if path != "" {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		record := records[path]

		splitPath := strings.Split(path, "/")
		// Get the end of the path, so we stop early
		filenameIndex := len(splitPath) - 1
		filename := splitPath[filenameIndex]
		splitPath = splitPath[:filenameIndex]

		parent := root
		pathSoFar := ""

		for _, pathSegment := range splitPath {
			pathSoFar = pathSoFar + "/" + pathSegment
			newParent, isOk := parent.children[pathSegment]
			if !isOk {
				panic("Directory or string wasn't reported: " + path)
			}
			parent = newParent
		}
		// current now holds the map we want to write this record into
		parent.children[filename] = Layout{filename: filename, completePath: path, description: record.description, children: make(map[string]Layout)}
	}
	return root
}

func printLayout(w *bufio.Writer, indent int, layout Layout) {
	if layout.filename != "" {
		indentStr := strings.Repeat(" ", indent)
		w.WriteString(indentStr)
		w.WriteString("- [")
		w.WriteString(layout.filename)
		w.WriteString("](")
		w.WriteString(layout.completePath)
		w.WriteString("):")
		if layout.description == "" || len(layout.description)+indent+len(layout.filename)+len(layout.completePath)+8 < 80 {
			w.WriteString(" ")
			w.WriteString(layout.description)
		} else {
			w.WriteString("\n")
			// TODO: make description readable in non-preview mode
			// lineLength := 80 - indent
			// descriptionLines := make([]string, 0)
			w.WriteString(indentStr)
			w.WriteString("  ")
			w.WriteString(layout.description)
		}
		w.WriteString("\n")
	}

	children := make([]string, 0)
	dotChildren := make([]string, 0)
	for child := range layout.children {
		if strings.HasPrefix(child, ".") {
			dotChildren = append(dotChildren, child)
		} else {
			children = append(children, child)
		}
	}
	sort.Strings(children)
	sort.Strings(dotChildren)
	children = append(children, dotChildren...)
	for _, child := range children {
		printLayout(w, indent+2, layout.children[child])
	}
}

func printLayouts(layout Layout) {
	f, err := os.Create("TOC.md")
	check("opening TOC.md", err)
	w := bufio.NewWriter(f)
	printLayout(w, -2, layout)
	w.Flush()
}

// -----------------------------
// Config
// -----------------------------

type DirectoriesValues = map[string]string

type Directories struct {
	NoListing []string          `yaml:nolisting`
	Values    DirectoriesValues `yaml:values`
}

type Config struct {
	Directories Directories
	Ignore      []string
}

func readConfig() Config {
	configString, err := ioutil.ReadFile("toc.yaml")
	if err == nil {
		var cfg Config
		err = yaml.Unmarshal(configString, &cfg)
		check("Reading config", err)
		fmt.Printf("config %+v\n", cfg)
		return cfg
	} else {
		var defaultDirs = Directories{NoListing: []string{}, Values: map[string]string{}}
		cfg := Config{Directories: defaultDirs, Ignore: []string{}}
		fmt.Printf("no config file %+v\n", cfg)
		return cfg
	}
}

// -----------------------------
// Main
// -----------------------------

func main() {
	// Read command line args
	flag.Parse()
	dir := flag.Arg(0)

	// Read config
	cfg := readConfig()

	// Pass 1: read the gitignores
	ignores := GitIgnores{}
	ignores.addBuiltin(dir)
	ignores.addList(cfg.Ignore)

	// Pass 2: get the metadata for the directory listing
	records := collectRecords(dir, &cfg, &ignores)

	// Reading the files is finished, so convert and print
	layout := convertRecordsToLayout(records)
	printLayouts(layout)

	fmt.Println("\nDone")
}
