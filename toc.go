package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
}

type Records = map[string]Record

func initializeGitIgnores(dir string) {
	fs := osfs.New(dir)
	patterns, err := gitignore.ReadPatterns(fs, []string{"."})
	check("reading gitignore files", err)
	ignoreMatcher = gitignore.NewMatcher(patterns)
	check("creating matchers from gitignore files", err)
}

func printRecords(records Records) {
	f, err := os.Create("TOC.md")
	check("opening TOC.md", err)
	w := bufio.NewWriter(f)

	for _, r := range records {
		w.WriteString("- [")
		w.WriteString(r.path)
		w.WriteString("](")
		w.WriteString(r.path)
		w.WriteString("): ")
		w.WriteString(r.description)
		w.WriteString("\n")
	}
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
			records[path] = Record{path: path, description: desc}
		} else {
			desc := "a file"
			records[path] = Record{path: path, description: desc}
		}
		return nil
	})
	check("Walking the directory tree", err)
	printRecords(records)

}
