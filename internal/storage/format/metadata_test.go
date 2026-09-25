package format

import "testing"

// TestMetadataBodyAgreesWithMetadataRegex holds the string fast path to the
// regexp it stands in for, including the blank, multiline, and overlapping
// delimiter lines it hands back to the regexp.
func TestMetadataBodyAgreesWithMetadataRegex(t *testing.T) {
	lines := []string{
		"<!-- short_id: 1, priority: 2 -->",
		"<!--short_id: 1-->",
		"<!-- a --> ",
		"<!-- a -->\t\n",
		"<!-- a--->",
		"<!-- --->",
		"<!-- -- -->",
		"<!-- a --> b -->",
		"<!---->",
		"<!--->",
		"<!-->",
		"<!--  -->",
		"<!-- \n -->",
		"<!-- a\nb -->",
		"<!--\na -->",
		"<!-- a\n-->",
		"<!-- a -->x",
		" <!-- a -->",
		"<!--  a  -->",
		"<!-- \va\v -->",
		"<!-- é, ünicode: → -->",
		"<!-- \xff -->",
		"-->",
		"",
	}
	for _, line := range lines {
		match := MetadataRegex().FindStringSubmatch(line)
		body, ok := MetadataBody(line)
		if (match != nil) != ok {
			t.Errorf("%q: regexp matched %v, MetadataBody %v", line, match != nil, ok)
			continue
		}
		if ok && body != match[1] {
			t.Errorf("%q: regexp captured %q, MetadataBody %q", line, match[1], body)
		}
	}
}

func FuzzMetadataBodyAgreesWithMetadataRegex(f *testing.F) {
	f.Add("<!-- short_id: 1, priority: 2 -->")
	f.Add("<!-- a--->")
	f.Add("<!-- \n -->")
	f.Fuzz(func(t *testing.T, line string) {
		match := MetadataRegex().FindStringSubmatch(line)
		body, ok := MetadataBody(line)
		if (match != nil) != ok || ok && body != match[1] {
			t.Fatalf("%q: regexp %q, MetadataBody %q %v", line, match, body, ok)
		}
	})
}
