package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
	"gopkg.in/yaml.v3"
)

func check(reason string, e error) {
	if e != nil {
		fmt.Printf("An unknown error occurred while %v\n", reason)
		panic(e)
	}
}

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
	gi.addList([]string{".git", ".gitkeep", ".gitattributes", ".dockerignore", "node_modules"})
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

type Record struct {
	path        string
	description string
	isFile      bool
}

type Records = map[string]Record

func writeRecords(records Records, path string, description string, isFile bool) {
	records[path] = Record{path: path, description: description, isFile: isFile}
}

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
		w.WriteString("): ")
		w.WriteString(layout.description)
		w.WriteString("\n")
	}

	children := make([]string, 0)
	for child := range layout.children {
		children = append(children, child)
	}
	sort.Strings(children)
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

type DirectoriesValues = map[string]string

type Directories struct {
	NoListing []string          `yaml:nolisting`
	Values    DirectoriesValues `yaml:values`
}

type Config struct {
	Directories Directories
	Ignore      []string
}

// var defaultDirs = Directories{IgnoreContents: []string{}}
var defaultDirs = Directories{}
var defaultConfig = Config{Directories: defaultDirs, Ignore: []string{}}

func main() {
	// Read command line args
	flag.Parse()
	dir := flag.Arg(0)

	// Read config
	configString, err := ioutil.ReadFile("toc.yaml")
	cfg := defaultConfig
	if err == nil {
		err = yaml.Unmarshal(configString, &cfg)
		check("Reading config", err)
		fmt.Printf("config %+v\n", cfg)
	} else {
		fmt.Printf("no config file %+v\n", cfg)
	}
	cfgNoListing := make(map[string]bool, len(cfg.Ignore))
	for _, dirName := range cfg.Directories.NoListing {
		cfgNoListing[dirName] = true
	}

	// Pass 1: read the gitignores
	ignores := GitIgnores{}
	ignores.addBuiltin(dir)
	ignores.addList(cfg.Ignore)

	// Pass 2: get the metadata for the directory listing
	records := make(map[string]Record)
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
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
			}
			writeRecords(records, pathname, desc, d.IsDir())
			if cfgNoListing[pathname] {
				fmt.Printf("nolisting: %q\n", pathname)
				return filepath.SkipDir
			}
		} else {
			desc := "a file"
			writeRecords(records, pathname, desc, d.IsDir())
		}
		return nil
	})
	check("Walking the directory tree", err)

	// Reading the files is finished, so convert and print
	layout := convertRecordsToLayout(records)
	printLayouts(layout)
	fmt.Println("\nDone")
}
