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

func check(e error) {
	if e != nil {
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
	check(err)
	ignoreMatcher = gitignore.NewMatcher(patterns)
	check(err)
}

func printRecords(records Records) {
	f, err := os.Create("TOC.md")
	check(err)
	w := bufio.NewWriter(f)

	for _, r := range records {
		w.WriteString(r.path)
		w.WriteString(": ")
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
		check(err)
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
		if d.IsDir() {
			desc := "a dir named " + path
			records[path] = Record{path: path, description: desc}
		} else {
			desc := "a file named " + path
			records[path] = Record{path: path, description: desc}
		}
		return nil
	})
	if err != nil {
		fmt.Printf("error walking the path %q: %v\n", '.', err)
		return
	} else {
		printRecords(records)
	}
}
