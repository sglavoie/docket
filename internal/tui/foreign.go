package tui

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/sigil"
)

// An outline kept in TaskPaper, OPML or Bike is read into the row grammar
// before import reads it, so a file from either comes in whole: its shape,
// its tasks and what is done. The file is told by its name.

// foreign is one row of such a file.
type foreign struct {
	depth int
	line  sigil.Line
}

// readForeign turns a file another outliner wrote into indented lines in the
// row grammar. Any other file is returned as it is.
func readForeign(path string, data []byte, today time.Time) (string, error) {
	var rows []foreign
	var err error
	switch strings.ToLower(filepath.Ext(path)) {
	case ".taskpaper":
		rows = readTaskPaper(string(data), today)
	case ".opml":
		rows, err = readOPML(data)
	case ".bike":
		rows, err = readBike(data)
	default:
		return string(data), nil
	}
	if err != nil {
		return "", errors.New("cannot read " + filepath.Base(path) + ": " + err.Error())
	}
	var b strings.Builder
	for _, r := range rows {
		// Every state is spelled, since the file's rows are tasks or notes
		// whatever they end up under.
		text := sigil.Format(r.line, r.line.State != "", today)
		if text == "" {
			text = sigil.NoteMark
		}
		if pastePrefix(text) {
			text = `\` + text
		}
		b.WriteString(strings.Repeat("\t", r.depth) + text + "\n")
	}
	return b.String(), nil
}

var taskPaperTag = regexp.MustCompile(`(^|\s)@([A-Za-z][\w.-]*)(\(([^)]*)\))?`)

// readTaskPaper reads TaskPaper's text: a line starting with a dash is a
// task, one ending in a colon a project, any other a note, and @name(value)
// words are tags. The tags Docket has a field for become that field.
func readTaskPaper(content string, today time.Time) []foreign {
	var out []foreign
	for _, raw := range strings.Split(strings.ReplaceAll(content, "\r", ""), "\n") {
		body := strings.TrimLeft(raw, "\t")
		if strings.TrimSpace(body) == "" {
			continue
		}
		row := foreign{depth: len(raw) - len(body)}
		l := &row.line
		body = strings.TrimSpace(body)
		task := strings.HasPrefix(body, "- ") || body == "-"
		if task {
			body, l.State = strings.TrimSpace(body[1:]), sigil.Open
		}
		body = taskPaperTag.ReplaceAllStringFunc(body, func(word string) string {
			m := taskPaperTag.FindStringSubmatch(word)
			name, value := strings.ToLower(m[2]), strings.TrimSpace(m[4])
			day := ""
			if f := strings.Fields(value); len(f) > 0 {
				day, _ = sigil.ParseDate(f[0], today)
			}
			switch {
			case name == "done":
				l.State = sigil.Done
			case name == "cancelled" || name == "canceled":
				l.State = sigil.Canceled
			case name == "flagged" || name == "flag":
				l.Flag = true
			case name == "due" && day != "":
				l.Due = day
			case (name == "start" || name == "defer") && day != "":
				l.Start = day
			default:
				tag := name
				if value != "" {
					tag += "=" + strings.Join(strings.Fields(value), "-")
				}
				tag, ok := sigil.ParseTag(tag)
				if !ok {
					return word // not a tag Docket can hold: it stays text
				}
				l.Tags = sigil.PutTag(l.Tags, tag)
			}
			return m[1]
		})
		l.Text = strings.Join(strings.Fields(body), " ")
		out = append(out, row)
	}
	return out
}

type opmlRow struct {
	Text     string    `xml:"text,attr"`
	Note     string    `xml:"_note,attr"`
	Status   string    `xml:"_status,attr"`
	Complete string    `xml:"complete,attr"`
	Rows     []opmlRow `xml:"outline"`
}

// readOPML reads an OPML outline. A row is a note unless the file says it
// is checked or not; a row's note becomes rows under it.
func readOPML(data []byte) ([]foreign, error) {
	var doc struct {
		Rows []opmlRow `xml:"body>outline"`
	}
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict, d.Entity = false, xml.HTMLEntity
	if err := d.Decode(&doc); err != nil {
		return nil, err
	}
	var out []foreign
	var walk func(rows []opmlRow, depth int)
	walk = func(rows []opmlRow, depth int) {
		for _, r := range rows {
			l := sigil.Line{Text: strings.Join(strings.Fields(r.Text), " ")}
			switch {
			case r.Status == "checked" || strings.EqualFold(r.Complete, "true"):
				l.State = sigil.Done
			case r.Status == "unchecked" || strings.EqualFold(r.Complete, "false"):
				l.State = sigil.Open
			}
			out = append(out, foreign{depth, l})
			for _, note := range strings.Split(r.Note, "\n") {
				if note = strings.Join(strings.Fields(note), " "); note != "" {
					out = append(out, foreign{depth + 1, sigil.Line{Text: note}})
				}
			}
			walk(r.Rows, depth+1)
		}
	}
	walk(doc.Rows, 0)
	return out, nil
}

// readBike reads a Bike outline, which is a nested list in HTML: each item
// holds a paragraph and the items under it. A task is a task, done when it
// says so; a heading gains the colon Docket reads one by; every other kind
// of row is a note, its formatting dropped.
func readBike(data []byte) ([]foreign, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict, d.Entity, d.AutoClose = false, xml.HTMLEntity, xml.HTMLAutoClose
	var out []foreign
	var open []int // the rows whose items are being read, outermost first
	var kinds []string
	inText := 0
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "li":
				var l sigil.Line
				kind := ""
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "data-type":
						kind = a.Value
					case "data-done":
						l.State = sigil.Done
					}
				}
				if kind == "task" && l.State == "" {
					l.State = sigil.Open
				} else if kind != "task" {
					l.State = ""
				}
				out = append(out, foreign{len(open), l})
				open, kinds = append(open, len(out)-1), append(kinds, kind)
			case "p":
				inText++
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "li":
				if n := len(open); n > 0 {
					l := &out[open[n-1]].line
					l.Text = strings.Join(strings.Fields(l.Text), " ")
					if kinds[n-1] == "heading" && l.Text != "" && !strings.HasSuffix(l.Text, ":") {
						l.Text += ":"
					}
					open, kinds = open[:n-1], kinds[:n-1]
				}
			case "p":
				inText = max(inText-1, 0)
			}
		case xml.CharData:
			if inText > 0 && len(open) > 0 {
				out[open[len(open)-1]].line.Text += string(t)
			}
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no rows in it")
	}
	return out, nil
}
