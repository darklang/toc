package main

// Entire toc program

import (
	"bufio"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	escape "github.com/JohannesKaufmann/html-to-markdown/escape"
	"github.com/integrii/flaggy"

	ignore "github.com/sabhiram/go-gitignore"
	"gopkg.in/yaml.v3"
)

// -----------------------------
// Builtin configuration data
// TODO: adding more here is always helpful
// -----------------------------
var builtinIgnores = []string{".git", ".gitkeep", ".gitignore", ".gitattributes", ".dockerignore", "node_modules", "README.md", "TOC.md"}

var uncommentedLanguages = []string{"eot", "woff", "ttf", "jpeg", "gif", "jpg", "pdf", "png"}

var builtinDescriptions = map[string]string{
	".circleci/config.yml": "CircleCI configuration",
	"dune":                 "Dune build",
	".fsproj":              "F# project",
	".csproj":              "F# project",
	"package.json":         "npm configuration",
	"go.mod":               "Go dependency management",
	"go.sum":               "Lockfile for go.mod",
	".toc.yaml":            "[toc](https://github.com/darklang/toc) configuration",
}

// -----------------------------
// Misc
// -----------------------------

func check(reason string, e error) {
	if e != nil {
		fmt.Printf("An unknown error occurred while %v\n", reason)
		panic(e)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
	gi.addList(builtinIgnores)
	rootIgnoreFile := root + "/.gitignore"
	if fileExists(rootIgnoreFile) {
		gi.addFile(rootIgnoreFile)
	}
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
				// If the prefix is repeated, keep removing it
				for strings.HasPrefix(line, c.prefix) {
					line = strings.TrimPrefix(line, c.prefix)
				}
				result = append(result, line)
			} else {
				break
			}
		} else {
			if strings.HasPrefix(line, c.prefix) {
				started = true
				// If the prefix is repeated, keep removing it
				for strings.HasPrefix(line, c.prefix) {
					line = strings.TrimPrefix(line, c.prefix)
				}
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

func s(prefix string) Syntax {
	return SingleLineComment{prefix: prefix}
}

func m(start string, end string) Syntax {
	return MultiLineComment{start: start, end: end}
}

var slashes = s("//")
var hash = s("#")
var cMulti = m("/*", "*/")
var javaMulti = m("/**", "*/")
var ocamlMulti1 = m("(*", "*)")
var ocamlMulti2 = m("(**", "*)")
var htmlMulti = m("<!--", "-->")

// Collections
var cStyle = []Syntax{slashes, cMulti}
var xmlStyle = []Syntax{htmlMulti}
var perlStyle = []Syntax{hash}
var defaultStyle = []Syntax{slashes, hash}
var javaStyle = append(cStyle, javaMulti)
var haskellStyle = []Syntax{s("--"), m("{-", "-}")}

var commentTable = map[string]([]Syntax){
	"bash":   perlStyle,
	"c":      cStyle,
	"cl":     []Syntax{s(";"), m("#|", "|#")}, // common lisp
	"clj":    []Syntax{s(";")},
	"coffee": perlStyle, // coffeescript
	"cpp":    cStyle,
	"cr":     perlStyle, // crystal
	"cs":     cStyle,    // C#
	"css":    []Syntax{cMulti},
	"cxx":    cStyle, // c++
	"d":      append(cStyle, m("/+", "+/")),
	"dart":   cStyle,
	"edn":    []Syntax{s(";")}, // clojure's edn data format
	"el":     []Syntax{s(";")}, // emacslisp
	"elm":    haskellStyle,
	"erl":    []Syntax{s("%")},
	"ex":     perlStyle, // elixir
	"exs":    perlStyle, // elixir
	"fish":   perlStyle,
	"fs":     []Syntax{slashes, ocamlMulti1, ocamlMulti2}, // F#
	"fsi":    []Syntax{slashes, ocamlMulti1, ocamlMulti2}, // F#
	"fsx":    []Syntax{slashes, ocamlMulti1, ocamlMulti2}, // F#
	"groovy": cStyle,
	"hs":     haskellStyle,
	"html":   xmlStyle,
	"hx":     cStyle, // haxe
	"java":   javaStyle,
	"jl":     append(perlStyle, m("#=", "=#")), // julia
	"js":     cStyle,
	"jsp":    javaStyle,
	"kt":     javaStyle, // kotlin
	"lisp":   []Syntax{s(";"), m("#|", "|#")},
	"lua":    []Syntax{s("--"), m("--[[", "]]")},
	"m":      cStyle, // objective-c
	"matlab": []Syntax{s("%")},
	"md":     perlStyle,
	"ml":     []Syntax{ocamlMulti1, ocamlMulti2},
	"nim":    []Syntax{hash, m("#[", "]#")},
	"php":    append(cStyle, hash),
	"pl":     perlStyle,
	"ps1":    []Syntax{hash, m("<#", "#>")}, // powershell
	"py":     []Syntax{hash, m("\"\"\"", "\"\"\"")},
	"r":      perlStyle,
	"rb":     []Syntax{hash, m("=begin", "=end")},
	"res":    cStyle,                                   // rescript
	"resi":   cStyle,                                   // rescript
	"rkt":    []Syntax{s(";"), m("#|", "|#"), s("#;")}, // racket
	"rss":    xmlStyle,
	"rs":     append(cStyle, m("/*!", "*/"), s(`//!`)),
	"sass":   cStyle,
	"scala":  cStyle,
	"scm":    []Syntax{s(";"), m("#|", "|#")}, // scheme
	"scss":   cStyle,
	"sh":     perlStyle,
	"sql":    []Syntax{s("--")},
	"st":     []Syntax{s("\"")}, // smalltalk
	"swift":  cStyle,
	"tex":    []Syntax{s("%")}, // tex/latex
	"ts":     []Syntax{slashes, cMulti},
	"vb":     []Syntax{s("'"), s("REM")},
	"vim":    []Syntax{s("\"")},
	"xml":    xmlStyle,
	"yaml":   perlStyle,
	"yml":    perlStyle,
	"zig":    []Syntax{s("///"), slashes},
	"zsh":    perlStyle,
}

// Look in the file and get the description from the first top-level comment
func getFileDescription(path string) (string, error) {
	firstFewLines := make([]string, 0)

	// Open the file for scanning
	reader, err := os.Open(path)
	if err != nil {
		return "", err
	}
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
	reader.Close()

	// Find the language
	extension := filepath.Ext(path)
	syntaxes, hasLanguage := commentTable[extension]
	if !hasLanguage {
		syntaxes = defaultStyle
	}

	// Get the description from the syntax definitions
	description := ""
	for _, syntax := range syntaxes {
		description = syntax.read(firstFewLines)
		if description != "" {
			break
		}
	}

	// Escape before returning so errant characters don't break the markdown syntax
	return escape.MarkdownCharacters(description), nil
}

// -----------------------------
// Records - store the data about each file
// -----------------------------

type Record struct {
	path        string
	description string
	isDir       bool
}

type Records = map[string]Record

func writeRecords(records Records, path string, description string, isDir bool) {
	records[path] = Record{path: path, description: description, isDir: isDir}
}

func collectRecords(dir string, cfg *Config, ignores *GitIgnores) Records {
	cfgNoListing := make(map[string]bool, len(cfg.Ignore))
	for _, dirName := range cfg.NoDirectoryContents {
		cfgNoListing[dirName] = true
	}

	records := make(map[string]Record)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		check("Walking into "+path, err)

		// If we find more ignores as we go on, add them to the matcher
		if strings.HasSuffix(path, "/.gitignore") {
			ignores.addFile(path)
			return nil
		}

		// Check if it's ignored via gitignore
		pathname := strings.TrimPrefix(path, dir)
		pathname = strings.TrimPrefix(pathname, "/")
		if ignores.Match(pathname) {
			// fmt.Printf("ignoring: %q\n", pathname)
			if d.IsDir() {
				return filepath.SkipDir
			} else {
				return nil
			}
		}

		desc := ""

		// Support builtin descriptions
		for defaultSuffix, defaultValue := range builtinDescriptions {
			if strings.HasSuffix(path, defaultSuffix) {
				desc = defaultValue
			}
		}

		// Use a default description if there's one in the config (overrides builtin)
		for defaultSuffix, defaultValue := range cfg.DefaultDescriptions {
			if strings.HasSuffix(path, defaultSuffix) {
				desc = defaultValue
			}
		}

		// Should we even read the file?
		shouldReadFromFile := desc == ""
		for _, extension := range uncommentedLanguages {
			if strings.HasSuffix(path, "."+extension) {
				shouldReadFromFile = false
			}
		}

		// Use a description for this file, if there's one in the config, else read from
		// the file (overrides builtin and default)
		cfgDesc, hasCfgDescription := cfg.Descriptions[pathname]
		if hasCfgDescription {
			desc = cfgDesc
		} else if shouldReadFromFile {
			if d.IsDir() {
				// If we can't find a README, ignore
				desc, _ = getFileDescription(path + "/" + "README.md")
			} else {
				desc, _ = getFileDescription(path)
			}
		}

		// Save description
		writeRecords(records, pathname, desc, d.IsDir())

		// Recurse
		if d.IsDir() && cfgNoListing[pathname] {
			return filepath.SkipDir
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
	isDir        bool
	description  string
	children     map[string]Layout
}

func convertRecordsToLayout(records Records) Layout {
	paths := make([]string, 0, len(records))

	root := Layout{completePath: "",
		filename:    "",
		description: "root",
		children:    make(map[string]Layout),
		isDir:       true}

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
		parent.children[filename] =
			Layout{
				filename:     filename,
				completePath: path,
				description:  record.description,
				isDir:        record.isDir,
				children:     make(map[string]Layout)}
	}
	return root
}

type Stats struct {
	missing          []string
	missingDirCount  int
	missingFileCount int
}

func (s *Stats) addMissingFile(path string) {
	s.missingFileCount++
	s.missing = append(s.missing, path)
}

func (s *Stats) addMissingDir(path string) {
	s.missingDirCount++
	s.missing = append(s.missing, path)
}

func printStats(w *strings.Builder, cfg *Config, stats *Stats) {
	w.WriteString("\n\n")
	w.WriteString("*Generated by [toc](https://github.com/darklang/toc)*")
	w.WriteString(" - *")
	if stats.missingDirCount > 0 || stats.missingFileCount > 0 {
		w.WriteString(strconv.Itoa(stats.missingDirCount))
		w.WriteString(" dirs and ")
		w.WriteString(strconv.Itoa(stats.missingFileCount))
		w.WriteString(" files still need descriptions*\n\n")
	} else {
		w.WriteString("the table of contents is fully specified!* ???? \n\n")
	}
}

func printLayout(w *strings.Builder, cfg *Config, stats *Stats, indent int, layout Layout) {
	// Keep some stats
	if layout.description == "" {
		if layout.isDir {
			stats.addMissingDir(layout.completePath)
		} else {
			stats.addMissingFile(layout.completePath)
		}
	}
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

	// Prioritize according to config, and then dotfiles at the end
	// TODO: the prioritized should be ordered by priority, not alphabetically
	prioritizedChildren := make([]string, 0)
	children := make([]string, 0)
	dotChildren := make([]string, 0)
	isInPriorityList := func(childFilename string) bool {
		var path string
		if layout.completePath == "" {
			path = childFilename
		} else {
			path = layout.completePath + "/" + childFilename
		}
		for _, priorityFilename := range cfg.ShowFirst {
			if path == priorityFilename {
				return true
			}
		}
		// fmt.Printf("Not prioritized %+v\n", childFilename)
		return false
	}
	for child := range layout.children {
		if strings.HasPrefix(child, ".") {
			dotChildren = append(dotChildren, child)
		} else if isInPriorityList(child) {
			prioritizedChildren = append(prioritizedChildren, child)
		} else {
			children = append(children, child)
		}
	}
	sort.Strings(prioritizedChildren)
	sort.Strings(children)
	sort.Strings(dotChildren)
	children = append(append(prioritizedChildren, children...), dotChildren...)
	for _, child := range children {
		printLayout(w, cfg, stats, indent+2, layout.children[child])
	}
}

func processLayouts(layout Layout, cfg *Config) (*Stats, string) {
	var w strings.Builder
	var stats Stats
	printLayout(&w, cfg, &stats, -2, layout)
	printStats(&w, cfg, &stats)
	return &stats, w.String()
}

// -----------------------------
// Config
// -----------------------------

type Config struct {
	NoDirectoryContents []string          `yaml:"noDirectoryContents"`
	Ignore              []string          `yaml:"ignore"`
	Descriptions        map[string]string `yaml:"descriptions"`
	DefaultDescriptions map[string]string `yaml:"defaultDescriptions"`
	ShowFirst           []string          `yaml:"showFirst"`
}

func readConfig(dir string) Config {
	configString, err := ioutil.ReadFile(dir + "/" + ".toc.yaml")
	if err == nil {
		var cfg Config
		err = yaml.Unmarshal(configString, &cfg)
		check("Reading config", err)
		// fmt.Printf("config %+v\n", cfg)
		return cfg
	} else {
		cfg := Config{
			Ignore:              []string{},
			NoDirectoryContents: []string{},
			Descriptions:        make(map[string]string),
			DefaultDescriptions: make(map[string]string),
			ShowFirst:           []string{}}

		// fmt.Printf("No config file %+v\n")
		return cfg
	}
}

// -----------------------------
// Main
// -----------------------------

func main() {

	showMissing := false
	dir := "./"
	buildCommand := flaggy.NewSubcommand("build")
	buildCommand.Description = "Build the table of contents"
	checkCommand := flaggy.NewSubcommand("check")
	checkCommand.Description = "Check the table of contents is up to date"
	flaggy.AttachSubcommand(checkCommand, 1)
	flaggy.AttachSubcommand(buildCommand, 1)
	buildCommand.AddPositionalValue(&dir, "dir", 1, false, "directory to create TOC.md from")
	checkCommand.AddPositionalValue(&dir, "dir", 1, false, "directory to create TOC.md from")
	buildCommand.Bool(&showMissing, "", "showMissing", "Show missing descriptions in files and directories")
	checkCommand.Bool(&showMissing, "", "showMissing", "Show missing descriptions in files and directories")
	flaggy.DefaultParser.AdditionalHelpPrepend = "Generate a table of contents for your repo"
	flaggy.Parse()

	// Read config
	cfg := readConfig(dir)

	// Pass 1: read the gitignores
	ignores := GitIgnores{}
	ignores.addBuiltin(dir)
	ignores.addList(cfg.Ignore)

	// Pass 2: get the metadata for the directory listing
	records := collectRecords(dir, &cfg, &ignores)

	// Reading the files is finished, so convert and print
	layout := convertRecordsToLayout(records)
	stats, layoutString := processLayouts(layout, &cfg)

	// Print it out
	if checkCommand.Used {
		current, err := ioutil.ReadFile(dir + "/TOC.md")
		if err != nil || string(current) != layoutString {
			fmt.Print("TOC.md is out of date")
			os.Exit(-1)
		}
	} else {
		// By default do a build
		f, err := os.Create(dir + "/TOC.md")
		check("opening TOC.md", err)
		w := bufio.NewWriter(f)
		w.WriteString(layoutString)
		w.Flush()
	}

	if showMissing {
		sort.Strings(stats.missing)
		for _, missing := range stats.missing {
			fmt.Printf("  %s\n", missing)
		}
	}
}
