package pandocconv

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestGetEnhancedFormat(t *testing.T) {
	got := getEnhancedFormat(MARKDOWN, true)
	if !strings.Contains(got, "backtick_code_blocks") {
		t.Fatalf("expected markdown enhancements, got %q", got)
	}
	if getEnhancedFormat(DOCX, true) != "docx+styles" {
		t.Fatalf("docx input should include +styles")
	}
	if getEnhancedFormat(DOCX, false) != "docx" {
		t.Fatalf("docx output should be plain docx")
	}
}

func TestBuildEnhancedArgsBasics(t *testing.T) {
	c := New(nil)
	args := c.buildEnhancedArgs(ConvertOptions{From: MARKDOWN, To: HTML5, PreserveFormatting: true, TableOfContents: true, NumberSections: true, DPI: 150, Metadata: map[string]string{"title": "MyDoc"}})

	joined := strings.Join(args, " ")
	for _, must := range []string{"--from=markdown+", "--to=html5+", "--wrap=preserve", "--preserve-tabs", "--toc", "--number-sections", "--dpi=150", "--metadata=title:MyDoc"} {
		if !strings.Contains(joined, must) {
			t.Fatalf("missing %q in args: %s", must, joined)
		}
	}
}

func TestCheckDependenciesRuns(t *testing.T) {
	c := New(nil)
	deps := c.CheckDependencies(context.Background())
	// Not asserting truthiness; just ensure it returns fast and is sane.
	if len(deps.Missing) == 0 && !deps.Pandoc {
		t.Fatalf("inconsistent state: no missing but pandoc=false")
	}
}

func TestCheckDependencies(t *testing.T) {
	err := CheckDependencies()
	if err != nil && !errors.Is(err, ErrDependencyMissing) {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Integration test: only runs if pandoc is present.
func TestConvertMarkdownToHTMLIfPandocPresent(t *testing.T) {
	if _, err := exec.LookPath("pandoc"); err != nil {
		t.Skip("pandoc not installed; skipping integration test")
	}
	c := New(nil)
	_, _ = c.CreateDefaultCSS()
	out, err := c.Convert(context.Background(), ConvertOptions{
		Content:            "# Title\n\n**bold** _italics_\n\n| A | B |\n| - | - |\n| 1 | 2 |",
		From:               MARKDOWN,
		To:                 HTML,
		PreserveFormatting: true,
		TableOfContents:    true,
		NumberSections:     true,
		Metadata:           map[string]string{"title": "Test"},
		Timeout:            20 * time.Second,
	})
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	if !strings.Contains(out, "<strong>bold</strong>") {
		t.Fatalf("expected HTML output to contain <strong>bold</strong>, got: %s", out)
	}
}
