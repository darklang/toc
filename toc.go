package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-billy/osfs"
	"gopkg.in/src-d/go-git.v4/plumbing/format/gitignore"
)

var ignoreMatcher gitignore.Matcher

func check(reason string, e error) {
	if e != nil {
		fmt.Printf("An unknown error occurred while %v\n", reason)
		panic(e)
	}
}

type Record struct {
	path        string
	description string
	isFile      bool
}

type Records = map[string]Record

// Recursive structure with all the results laid out, ready to print
type Layout struct {
	completePath string
	filename     string
	description  string
	children     map[string]Layout
}

func initializeGitIgnores(dir string) {
	fs := osfs.New(dir)
	patterns, err := gitignore.ReadPatterns(fs, []string{"."})
	check("reading gitignore files", err)
	ignoreMatcher = gitignore.NewMatcher(patterns)
	check("creating matchers from gitignore files", err)
}

func convertRecordsToLayout(records Records) Layout {
	paths := make([]string, 0, len(records))

	root := Layout{completePath: "", filename: "", description: "root", children: make(map[string]Layout, 0)}

	for path := range records {
		if path != "" {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		record := records[path]

		splitPath := strings.Split(path, "/")
		fmt.Printf("split path: %q\n", splitPath)
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

func writeRecords(records Records, path string, description string, isFile bool) {
	records[path] = Record{path: path, description: description, isFile: isFile}
}

func printLayout(w *bufio.Writer, indent int, layout Layout) {
	indentStr := strings.Repeat(" ", indent)
	w.WriteString(indentStr)
	w.WriteString("- [")
	w.WriteString(layout.filename)
	w.WriteString("](")
	w.WriteString(layout.completePath)
	w.WriteString("): ")
	w.WriteString(layout.description)
	w.WriteString("\n")
	for _, child := range layout.children {
		printLayout(w, indent+2, child)
	}
}

func printLayouts(layout Layout) {
	f, err := os.Create("TOC.md")
	check("opening TOC.md", err)
	w := bufio.NewWriter(f)
	printLayout(w, 0, layout)
	w.Flush()
}

func main() {
	flag.Parse()
	dir := flag.Arg(0)
	records := make(map[string]Record)

	initializeGitIgnores(dir)

	fmt.Printf("reading %q\n", dir)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		check("Walking into "+path, err)
		path = strings.TrimPrefix(path, dir)
		path = strings.TrimPrefix(path, "/")

		// Check if it's ignored
		if ignoreMatcher.Match([]string{path}, d.IsDir()) || path == ".git" {
			fmt.Printf("skipping: %q\n", path)
			if d.IsDir() {
				return filepath.SkipDir
			} else {
				return nil
			}
		}

		// Save description
		if d.IsDir() {
			desc := "a dir"
			writeRecords(records, path, desc, d.IsDir())
		} else {
			desc := "a file"
			writeRecords(records, path, desc, d.IsDir())
		}
		return nil
	})
	check("Walking the directory tree", err)
	layout := convertRecordsToLayout(records)
	printLayouts(layout)

}
