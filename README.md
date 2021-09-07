# Build a table of contents for your repo

Briefly, TOC:

- creates a table of contents for your repo in `TOC.md`
- new contributors can read this to get a sense of what's going on
- descriptions are created using the first comment in each file (or `README.md` for directories)
- TOC tells you when files do not have a description

# Installation

Currently, you must install from source. Run `go build .` to build the `toc` binary.

Adding TOC to major package managers would be welcome.

# Usage

## Create a TOC file

```
toc build
```

This creates a TOC.md file. You should check this into your repo for others to read.

You can automatically update this with a git hook: [toc-pre-commit.sh](toc-pre-commit.sh).

## Customize your TOC file

Add a `.toc.yaml` file to your repo, this will then be used to configure the TOC
output. Here is an example from the [Dark repo](https://github.com/dark/darklang):

```

# You can add descriptions directly, useful for when you can't edit a file or directory
descriptions:
  _esy: "Build directory used by esy, the OCaml package manager"
  _build: "Build directory for OCaml"
  esy.lock: "Lockfiles for esy, the OCaml package manager"
  client/static/vendor/fontawesome-5.12.0: "Vendored font-awesome install"

# Files with this suffix get automatic descriptions. Use for common things that don't
# need to be addressed each time
defaultComments:
  .fsproj: "Project file"
  paket.references: "Dependencies"


# Uses a gitignore glob
ignore:
  - .merlin
  - /CHANGELOG.md

# Show the directory, but do not recurse into their contents. Use for directories
# into which you can place a README.md, but listing the contents is not useful.
directoryOnly:
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

# Move these up to the top of the listing. Nested files will be moved to the top of
# the nested listing. Use this for the most important files and directories for
# contributors to read.
showFirst:
  - client
  - fsharp-backend
  - fsharp-backend/src
  - fsharp-backend/tests

```

## Validate your TOC file is up-to-date

```
toc check
```

This is useful in [CI](https://circleci.com).

## Improve the descriptions

By default, TOC will include a count of files and directories without descriptions at
the bottom of `TOC.md`. This is helpful during code review to ensure this number
doesn't sneak upwards.

Use `toc build --list-missing` to give a list of files without descriptions.
