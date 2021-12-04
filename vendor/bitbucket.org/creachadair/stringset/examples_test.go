package stringset_test

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"bitbucket.org/creachadair/stringset"
)

func ExampleSet_Intersect() {
	a := stringset.New("one", "two", "three")
	b := stringset.New("two", "four", "six")
	fmt.Println(a.Intersect(b))
	// Output: {"two"}
}

func ExampleSet_Union() {
	s := stringset.New("0", "1", "2").Union(stringset.New("x"))
	fmt.Println(s)
	// Output: {"0", "1", "2", "x"}
}

func ExampleSet_Discard() {
	nat := stringset.New("0", "1", "2", "3", "4")
	ok := nat.Discard("2", "4", "6")
	fmt.Println(ok, nat)
	// Output: true {"0", "1", "3"}
}

func ExampleSet_Add() {
	s := stringset.New("A", "B")
	s.Add("B", "C", "D")
	fmt.Println(s)
	// Output: {"A", "B", "C", "D"}
}

func ExampleSet_Select() {
	re := regexp.MustCompile(`[a-z]\d+`)
	s := stringset.New("a", "b15", "c9", "q").Select(re.MatchString)
	fmt.Println(s)
	// Output: {"b15", "c9"}
}

func ExampleSet_Choose() {
	s := stringset.New("a", "ab", "abc", "abcd")
	long, ok := s.Choose(func(c string) bool {
		return len(c) > 3
	})
	fmt.Println(long, ok)
	// Output: abcd true
}

func ExampleSet_Contains() {
	s := stringset.New("a", "b", "c", "d", "e")
	ae := s.Contains("a", "e")       // all present
	bdx := s.Contains("b", "d", "x") // x missing
	fmt.Println(ae, bdx)
	// Output: true false
}

func ExampleSet_ContainsAny() {
	s := stringset.New("a", "b", "c")
	fm := s.ContainsAny("f", "m")       // all missing
	bdx := s.ContainsAny("b", "d", "x") // b present
	fmt.Println(fm, bdx)
	// Output: false true
}

func ExampleSet_Diff() {
	a := stringset.New("a", "b", "c")
	v := stringset.New("a", "e", "i")
	fmt.Println(a.Diff(v))
	// Output: {"b", "c"}
}

func ExampleSet_Each() {
	sum := 0
	stringset.New("one", "two", "three").Each(func(s string) {
		sum += len(s)
	})
	fmt.Println(sum)
	// Output: 11
}

func ExampleSet_Pop() {
	s := stringset.New("a", "bc", "def", "ghij")
	p, ok := s.Pop(func(s string) bool {
		return len(s) == 2
	})
	fmt.Println(p, ok, s)
	// Output: bc true {"a", "def", "ghij"}
}

func ExampleSet_Partition() {
	s := stringset.New("aba", "d", "qpc", "ff")
	a, b := s.Partition(func(s string) bool {
		return s[0] == s[len(s)-1]
	})
	fmt.Println(a, b)
	// Output: {"aba", "d", "ff"} {"qpc"}
}

func ExampleSet_SymDiff() {
	s := stringset.New("a", "b", "c")
	t := stringset.New("a", "c", "t")
	fmt.Println(s.SymDiff(t))
	// Output: {"b", "t"}
}

func ExampleContains_slice() {
	s := strings.Fields("four fine fat fishes fly far")
	fmt.Println(stringset.Contains(s, "fishes"))
	// Output:
	// true
}

func ExampleContains_map() {
	s := map[string]int{"apples": 12, "pears": 2, "plums": 0, "cherries": 18}
	fmt.Println(stringset.Contains(s, "pears"))
	// Output:
	// true
}

func ExampleContains_set() {
	s := stringset.New("lead", "iron", "copper", "chromium")
	fmt.Println(stringset.Contains(s, "chromium"))
	// Output:
	// true
}

func ExampleFromKeys() {
	s := stringset.FromKeys(map[string]int{
		"one":   1,
		"two":   2,
		"three": 3,
	})
	fmt.Println(s)
	// Output: {"one", "three", "two"}
}

func ExampleFromIndexed() {
	type T struct {
		Event       string
		Probability float64
	}
	events := []T{
		{"heads", 0.625},
		{"tails", 0.370},
		{"edge", 0.005},
	}
	s := stringset.FromIndexed(len(events), func(i int) string {
		return events[i].Event
	})
	fmt.Println(s)
	// Output: {"edge", "heads", "tails"}
}

func ExampleFromValues() {
	s := stringset.FromValues(map[int]string{
		1: "red",
		2: "green",
		3: "red",
		4: "blue",
		5: "green",
	})
	fmt.Println(s)
	// Output: {"blue", "green", "red"}
}

func ExampleIndex() {
	s := strings.Fields("full plate and packing steel")
	fmt.Println(stringset.Index("plate", s))
	fmt.Println(stringset.Index("spoon", s))
	// Output:
	// 1
	// -1
}

func ExampleSet_Map() {
	names := stringset.New("stdio.h", "main.cc", "lib.go", "BUILD", "fixup.py")
	fmt.Println(names.Map(filepath.Ext))
	// Output:
	// {"", ".cc", ".go", ".h", ".py"}
}
