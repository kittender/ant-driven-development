package instruments

import (
	"encoding/xml"
	"fmt"
)

type junitSuites struct {
	Suites []struct {
		Name  string `xml:"name,attr"`
		Cases []struct {
			Name      string `xml:"name,attr"`
			Classname string `xml:"classname,attr"`
			Failure   *struct {
				Message string `xml:"message,attr"`
				Type    string `xml:"type,attr"`
				Body    string `xml:",chardata"`
			} `xml:"failure"`
			Error *struct {
				Message string `xml:"message,attr"`
			} `xml:"error"`
		} `xml:"testcase"`
	} `xml:"testsuite"`
}

// JUnitMessage is the error message of one test case, from the JUnit report.
type JUnitMessage struct {
	File, Name, Message string
}

// ParseJUnit reads Vitest's JUnit report: one entry per test case, in order,
// with its failure message ("" when it passed).
func ParseJUnit(root string, out []byte) ([]JUnitMessage, error) {
	var r junitSuites
	if err := xml.Unmarshal(out, &r); err != nil {
		return nil, fmt.Errorf("junit xml: %w", err)
	}
	var ms []JUnitMessage
	for _, s := range r.Suites {
		for _, c := range s.Cases {
			m := JUnitMessage{File: rel(root, s.Name), Name: c.Name}
			if c.Classname != "" {
				m.File = rel(root, c.Classname)
			}
			switch {
			case c.Failure != nil && c.Failure.Message != "":
				m.Message = c.Failure.Message
			case c.Failure != nil:
				m.Message = c.Failure.Type
			case c.Error != nil:
				m.Message = c.Error.Message
			}
			ms = append(ms, m)
		}
	}
	return ms, nil
}

// MergeMessages puts the JUnit message in front of each failed case's stack,
// pairing cases by file and position (outline examples share one title, so
// the order is the only reliable key; both reports list cases in run order).
func MergeMessages(cases []TestCase, msgs []JUnitMessage) {
	byFile := map[string][]JUnitMessage{}
	for _, m := range msgs {
		byFile[baseOf(m.File)] = append(byFile[baseOf(m.File)], m)
	}
	next := map[string]int{}
	for i, c := range cases {
		f := baseOf(c.File)
		q := byFile[f]
		j := next[f]
		if j >= len(q) {
			continue
		}
		next[f] = j + 1
		if q[j].Message != "" && c.Status != "passed" {
			cases[i].Failure = trim(q[j].Message + "\n" + c.Failure)
		}
	}
}

func baseOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}
