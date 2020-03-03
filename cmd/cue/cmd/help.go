// Copyright 2020 CUE Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"github.com/spf13/cobra"
)

func newHelpTopics(c *Command) []*cobra.Command {
	return []*cobra.Command{
		packagesHelp,
		flagsHelp,
		filetypeHelp,
	}
}

var packagesHelp = &cobra.Command{
	Use:   "packages",
	Short: "package list, patterns, and files",
	Long: `Many commands apply to a set of packages:

cue <command> [packages]

The list of [packages] may specify CUE packages, CUE files,
non-CUE files or a combination of those. An empty list specifies
the package in the current directory, provided there is a single
named package in this directory.

CUE packages are specified as an import path. An import path
that is a rooted path --one that begins with a "." or ".."
element-- is interpreted as a file system path and denotes the
package instance in that directory.

Otherwise, the import path denotes an external package found
in one of the cue.mod subdirectories.

An import path may contain a ... to match any subdirectory:
pkg/... matches all packages below pkg, including pkg itself,
while foo/.../bar matches all directories named bar within foo.
In all cases, cue.mod directories are excluded from the result.

A package may also be specified as a list of .cue files.
The special symbol '-' denotes stdin or stdout and defaults to
the cue file type for stdin. For stdout, the default depends on
the cue command. A .cue file package may not be combined with
regular packages.

Non-cue files are interpreted based on their file extension or,
if present, an explicit file qualifier (see the "filetype"
help topic). Non-cue files may be interpreted as data or schema.
Schema are treated as single-file packages by default. See the
"filetype" and "flags" help topics on how to combine schema
into a single package.

Data files can be combined into a single package, in which case
each file is unified into a defined location within the target
package. If a data file has multiple values, such as allowed
with JSON Lines or YAML, each value is interpreted as a separate
file.

The --schema/-d flag can be used to unify each data file against
a specific schema within a non-data package. For OpenAPI, the -d
flag specifies a schema name. For JSON Schema the -d flag
specifies a schema defined in "definitions". In all other cases,
the -d flag is a CUE expression that is evaluated within the
package.

Examples (also see also "flags" and "filetype" help topics):

# Show the definition of each package named foo for each
# directory dir under path.
$ cue def ./path/.../dir:foo

# Unify each document in foo.yaml with the value Foo in pkg.
$ cue export ./pkg -d Foo foo.yaml

# Unify data.json with schema.json.
$ cue export data.json schema: schema.json
`,
}

var flagsHelp = &cobra.Command{
	Use:   "flags",
	Short: "common flags for composing packages",
	Long: `Non-CUE files are treated as individual files by
default, but can be combined into a single package using a
combination of the following flags.


Assigning values to a CUE path

The --path/-l flag can be used to specify a CUE path at which
to place a value. Each -l flag specifies either a CUE
expression or a CUE field (without the final value), both
of which are evaluated within the value. Together, the -l flags
specify the path at increasingly deeper nesting. The path
notation can be used to indicate a field must be a definition.
An expression may refer to builtin packages as long as the
name can be uniquely identified.

The --with-context flag can be used to evaluate the label
expression within a struct of contextual data, instead of
within the value itself. This struct has the following fields:

{
	// data holds the original source data
	// (perhaps one of several records in a file).
	data: _
	// filename holds the full path to the file.
	filename: string
	// index holds the 0-based index element of the
	// record within the file. For files containing only
	// one record, this will be 0.
	index: uint & <recordCount
	// recordCount holds the total number of records
	// within the file.
	recordCount: int & >=1
}


Handling multiple documents or streams

To handle multi-document files, such as JSON Lines or YAML
files with document separators (---), the user must specify
a the --path, --list, or --files flag.
The --path flag merges each element into a single package as
if each element was defined in a separate file. The --list flag
concatenates each entry in a file into a list.
Using --list flag in combination with the --path flag
concatenates entries with the same path into a list, instead of
unifying them.
Finally, the --files option causes each entry to be written to
a different file. The -files flag may only be used in
combination with the import command.


Examples:

# Put a value at a path based on its "kind" and "name" fields.
$ cue eval -l 'strings.ToLower(kind)' -l name foo.yaml

# Include a schema under the "myschema" field using the path notation.
$ cue eval -l myschema: schema: foo.json

# Base the path values on its kind and file name.
$ cue eval --with-context -l 'path.Base(filename)' -l data.kind foo.yaml
`,
}

var filetypeHelp = &cobra.Command{
	Use:   "filetype",
	Short: "supported file types and qualifiers",
	Long: `The cue tools supports the following file types:

    Tag        Extensions       Description
    cue        .cue             CUE source files.
    json       .json            JSON files.
    yaml       .yaml/.yml       YAML files.
    jsonl      .jsonl/.ldjson   Line-separated JSON values.
    jsonschema                  JSON Schema.
    openapi                     OpenAPI schema.
    proto      .proto           Protocol buffer definition files.
    go         .go              Go source files.
	text       .txt             Raw text file; the evaluated
	                            value must be of type string.

The cue tool will infer a file's type from its extension
by default. The user my override this behavior by using
qualifiers. A qualifier takes the form

    <tag>{'+'<tag>}':'

For instance,

    cue eval json: foo.data

specifies that 'foo.data' should be read as a JSON file.
A qualifier applies to all files following on the command
line until the next qualifier. The cue tool does not allow
a ':' in filenames.

File formats that do not have a default extension may be
represented in any data format and need a qualifier. For
instance:

   cue def openapi: bar.cue foo.yaml

interprets the files bar.cue and foo.yaml as data in the
respective formats representing an OpenAPI definition.

The following tags can be used in qualifiers to further
influence input or output. For input these act as
restrictions, validating the input. For output these act
as filters, showing only the requested data and picking
defaults as requested.

    Tag         Description
    data        require concrete input and output
    graph       like data, but allow references and anchors
    schema      export data and definitions

Many commands also support the --out and --outfile/-o flags.
The --out flag specifies the output type using a qualifier
(without the ':'). The -o flag specifies an output file
possibly prefixed with a qualifier.

Examples:

# Interpret bar.cue and foo.yaml as OpenAPI data.
$ cue def openapi: bar.cue foo.yaml

# Write a CUE package as OpenAPI encoded as YAML, using
# an alternate file extension.
$ cue def -o openapi+yaml:foo.openapi

# Print the data for the current package as YAML.
$ cue export --out=yaml

# Print the string value of the "name" field as a string.
$ cue export -e name --out=text

# Write the string value of the "name" field to a text file.
$ cue export -e name -o=foo.txt

# Write the string value of the "name" field to a file foo.
$ cue export -e name -o=text:foo
`,
}

// TODO: tags
// - doc/nodoc
// - attr/noattr
// - id=<url>

// TODO: filetypes:
// - textpb
// - binpb

// TODO: document
// <tag>['='<value>]{'+'<tag>['='<value>]}':'

// TODO: cue.mod help topic
