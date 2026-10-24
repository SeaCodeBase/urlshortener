package service

import "testing"

func TestCSVSafe_PrefixesFormulaChars(t *testing.T) {
	for in, want := range map[string]string{
		`=HYPERLINK("x")`: `'=HYPERLINK("x")`,
		"+1":              "'+1",
		"-1":              "'-1",
		"@SUM(A1)":        "'@SUM(A1)",
		"\tx":             "'\tx",
		"\rx":             "'\rx",
		"https://e.com":   "https://e.com",
		"":                "",
		"a=b":             "a=b",
	} {
		if got := csvSafe(in); got != want {
			t.Errorf("csvSafe(%q) = %q, want %q", in, got, want)
		}
	}
}
