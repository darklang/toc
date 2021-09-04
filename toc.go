package main

import (
	"flag"
	"fmt"
	"io/fs"
	"path/filepath"

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

func initializeGitIgnores(dir string) {
	fs := osfs.New(dir)
	patterns, err := gitignore.ReadPatterns(fs, []string{"."})
	check(err)
	ignoreMatcher = gitignore.NewMatcher(patterns)
	check(err)
}

var records map[string]Record

func writeRecord(path string, description string) {
	records[path] = Record{path: path, description: description}
}

func main() {
	flag.Parse()
	dir := flag.Arg(0)
	records = make(map[string]Record)

	initializeGitIgnores(dir)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		check(err)
		// Check if it's ignored
		if ignoreMatcher.Match([]string{path}, d.IsDir()) {
			fmt.Printf("skipping: %q\n", path)
			if d.IsDir() {
				return filepath.SkipDir
			} else {
				return nil
			}
		}
		if d.IsDir() {
			desc := "a dir named" + path
			writeRecord(path, desc)
		} else {
			desc := "a file named" + path
			writeRecord(path, desc)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("error walking the path %q: %v\n", '.', err)
		return
	} else {
		fmt.Printf("done %v\n", records)
	}
}
