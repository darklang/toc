# Build a table of contents for your repo

Briefly, `toc`:

- creates a table of contents for your repo in `TOC.md`
- new contributors can read this to get a sense of what's going on
- descriptions are created using the first comment in each file (or `README.md` for directories)
- `toc` tells you when files are missing description

# Installation

Currently, you must install from source. Run `go build .` to build the `toc` binary.

Adding `toc` to major package managers would be welcome.

# Usage

## Create a `TOC.md` file

```
toc build
```

This creates a `TOC.md` file. You should check this into your repo for others to read.

You can automatically update this with a git hook: [toc-pre-commit.sh](toc-pre-commit.sh).

## Customize your `TOC.md` file

Add a `.toc.yaml` file to your repo, this will then be used to configure `toc`'s
output. Here is an example from the [Dark repo](https://github.com/dark/darklang):

```
# You can add descriptions directly, useful for when you can't edit a file or directory
descriptions:
  _esy: "Build directory used by esy, the OCaml package manager"
  _build: "Build directory for OCaml"
  esy.lock: "Lockfiles for esy, the OCaml package manager"
  client/static/vendor/fontawesome-5.12.0: "Vendored font-awesome install"

# Do not display these at all - uses a gitignore glob
ignore:
  - .merlin
  - .ocamlformat
  - /CHANGELOG.md
  - /CODE-OF-CONDUCT.md
  - /CODING-GUIDE.md
  - /LICENSE.md
  - /LICENSES


# Move these up to the top of the listing. Nested files will be moved to the top of
# the nested listing. Use this for the most important files and directories for
# contributors to read.
showFirst:
  - client
  - fsharp-backend
  - fsharp-backend/src
  - fsharp-backend/tests

# Files with these suffixes get automatic descriptions. Use for common things that
# don't need to be addressed each time
defaultDescriptions:
  .fsproj: "Project file"
  .unported: "Unported OCaml file"
  paket.references: "Dependencies"

# Show the directory, but do not recurse into their contents.
noDirectoryContents:
  - _build
  - _esy
  - auth0-branding
  - backend/migrations
  - backend/serialization
  - backend/static/blazor
  - backend/test_appdata
  - client/static/vendor/fontawesome-5.12.0
  - esy.lock
  - fsharp-backend/tests/httpclienttestfiles
  - fsharp-backend/tests/httptestfiles

```

## Validate your `TOC.md` is up-to-date

```
toc check
```

This is useful in [CI](https://circleci.com).

## Improve the descriptions

By default, `toc` will include a count of files and directories without descriptions
at the bottom of `TOC.md`. This is helpful during code review to ensure this number
doesn't sneak upwards.

Use `toc build --list-missing` to give a list of files without descriptions.
