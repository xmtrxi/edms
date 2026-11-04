package service

import (
	"slices"
	"strings"
	"testing"
)

func TestCitedIndexes(t *testing.T) {
	got := citedIndexes("Budget is 5M [2]. Approved by the board [1][2]. Ignore [9] and [0].", 3)
	if !slices.Equal(got, []int{2, 1}) {
		t.Fatalf("got %v", got)
	}
}

func TestAnswerPromptNumbersSources(t *testing.T) {
	prompt := answerPrompt("What is the budget?", []passage{
		{Citation: Citation{DocumentName: "Budget.pdf", PageStart: 3}, text: "Total 5M"},
		{Citation: Citation{DocumentName: "Minutes.docx", PageStart: 1, PageEnd: 2}, text: "Approved"},
	})
	for _, want := range []string{"[1] Budget.pdf (page 3)", "[2] Minutes.docx (pages 1–2)", "Question: What is the budget?"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt is missing %q:\n%s", want, prompt)
		}
	}
}
