package main

import (
	"testing"
)

func TestLineIsExcluded(t *testing.T) {
	t.Parallel()

	var tests = []struct {
		line     string
		excludes []string
		want     bool
	}{
		{line: "error 1", excludes: []string{}, want: false},
		{line: "error 2", excludes: []string{"error"}, want: true},
		{line: "error 3", excludes: []string{"1", "2", "3", "error"}, want: true},
		{line: "error 4", excludes: []string{"1", "2", "3", "4"}, want: true},
		{line: "error 5", excludes: []string{"1", "2", "3", "4"}, want: false},
		{line: "error 6", excludes: []string{"6"}, want: true},
		{line: "error 7", excludes: []string{"1"}, want: false},
	}
	for _, tt := range tests {
		tt := tt // NOTE: https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables
		t.Run(tt.line, func(t *testing.T) {
			t.Parallel()
			f := configFile{
				FileName: "asdf.txt",
				Excludes: tt.excludes,
			}
			got := lineIsExcluded(f, tt.line)
			if got != tt.want {
				t.Errorf("want %t, got %t", tt.want, got)
			}
		})
	}
}
