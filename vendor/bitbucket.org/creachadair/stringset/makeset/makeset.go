// Program makeset generates source code for a set package.  The type of the
// elements of the set is determined by a TOML configuration stored in a file
// named by the -config flag.
//
// Usage:
//   go run makeset.go -output $DIR -config config.toml
//
package main

//go:generate go run github.com/creachadair/staticfile/compiledata -out static.go *.go.in

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/creachadair/staticfile"
)

// A Config describes the nature of the set to be constructed.
type Config struct {
	// A human-readable description of the set this config defines.
	// This is ignored by the code generator, but may serve as documentation.
	Desc string

	// The name of the resulting set package, e.g., "intset" (required).
	Package string

	// The name of the type contained in the set, e.g., "int" (required).
	Type string

	// The spelling of the zero value for the set type, e.g., "0" (required).
	Zero string

	// If set, a type definition is added to the package mapping Type to this
	// structure, e.g., "struct { ... }". You may prefix Decl with "=" to
	// generate a type alias (this requires Go ≥ 1.9).
	Decl string

	// If set, the body of a function with signature func(x, y Type) bool
	// reporting whether x is less than y.
	//
	// For example:
	//   if x[0] == y[0] {
	//     return x[1] < y[1]
	//   }
	//   return x[0] < y[0]
	Less string

	// If set, the body of a function with signature func(x Type) string that
	// converts x to a human-readable string.
	//
	// For example:
	//   return strconv.Itoa(x)
	ToString string

	// If set, additional packages to import in the generated code.
	Imports []string

	// If set, additional packages to import in the test.
	TestImports []string

	// If true, include transformations, e.g., Map, Partition, Each.
	Transforms bool

	// A list of exactly ten ordered test values used for the construction of
	// unit tests. If omitted, unit tests are not generated.
	TestValues []interface{} `json:"testValues,omitempty"`
}

func (c *Config) validate() error {
	if c.Package == "" {
		return errors.New("invalid: missing package name")
	} else if c.Type == "" {
		return errors.New("invalid: missing type name")
	} else if c.Zero == "" {
		return errors.New("invalid: missing zero value")
	}
	return nil
}

var (
	configPath = flag.String("config", "", "Path of configuration file (required)")
	outDir     = flag.String("output", "", "Output directory path (required)")

	baseImports = []string{"reflect", "sort", "strings"}
)

func main() {
	flag.Parse()
	switch {
	case *outDir == "":
		log.Fatal("You must specify a non-empty -output directory")
	case *configPath == "":
		log.Fatal("You must specify a non-empty -config path")
	}
	conf, err := readConfig(*configPath)
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}
	if len(conf.TestValues) > 0 && len(conf.TestValues) != 10 {
		log.Fatalf("Wrong number of test values (%d); exactly 10 are required", len(conf.TestValues))
	}
	if err := os.MkdirAll(*outDir, 0755); err != nil {
		log.Fatalf("Unable to create output directory: %v", err)
	}

	mainT, err := template.New("main").Parse(string(staticfile.MustReadFile("core.go.in")))
	if err != nil {
		log.Fatalf("Invalid main source template: %v", err)
	}
	testT, err := template.New("test").Parse(string(staticfile.MustReadFile("core_test.go.in")))
	if err != nil {
		log.Fatalf("Invalid test source template: %v", err)
	}

	mainPath := filepath.Join(*outDir, conf.Package+".go")
	if err := generate(mainT, conf, mainPath); err != nil {
		log.Fatal(err)
	}
	if len(conf.TestValues) != 0 {
		testPath := filepath.Join(*outDir, conf.Package+"_test.go")
		if err := generate(testT, conf, testPath); err != nil {
			log.Fatal(err)
		}
	}
}

// readConfig loads a configuration from the specified path and reports whether
// it is valid.
func readConfig(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := toml.Unmarshal(data, &c); err != nil {
		return nil, err
	}

	// Deduplicate the import list, including all those specified by the
	// configuration as well as those needed by the static code.
	imps := make(map[string]bool)
	for _, pkg := range baseImports {
		imps[pkg] = true
	}
	for _, pkg := range c.Imports {
		imps[pkg] = true
	}
	if c.ToString == "" {
		imps["fmt"] = true // for fmt.Sprint
	}
	c.Imports = make([]string, 0, len(imps))
	for pkg := range imps {
		c.Imports = append(c.Imports, pkg)
	}
	sort.Strings(c.Imports)
	return &c, c.validate()
}

// generate renders source text from t using the values in c, formats the
// output as Go source, and writes the result to path.
func generate(t *template.Template, c *Config, path string) error {
	var buf bytes.Buffer
	if err := t.Execute(&buf, c); err != nil {
		return fmt.Errorf("generating source for %q: %v", path, err)
	}
	src, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("formatting source for %q: %v", path, err)
	}
	return ioutil.WriteFile(path, src, 0644)
}
