
package pandocconv

import "strings"

// Input formats
const (
	BIBLATEX = "biblatex"
	BIBTEX   = "bibtex"
	CSV      = "csv"
	DOCX     = "docx"
	EPUB     = "epub"
	FB2      = "fb2"
	HTML     = "html"
	HTML5    = "html5"
	IPYNB    = "ipynb"
	JSON     = "json"
	LATEX    = "latex"
	MARKDOWN = "markdown"
	ODT      = "odt"
	RIS      = "ris"
	RTF      = "rtf"
	TSV      = "tsv"
	TEXTILE  = "textile"
	VIMWIKI  = "vimwiki"
)

// Output formats (subset for brevity; extend as needed)
const (
	ASCIIDOC = "asciidoc"
	BEAMER   = "beamer"
	DOCBOOK  = "docbook"
	GFM      = "gfm"
	JATS     = "jats"
	MAN      = "man"
	MARKUA   = "markua"
	NATIVE   = "native"
	PDF      = "pdf"
	PLAIN    = "plain"
	PPTX     = "pptx"
	REVEALJS = "revealjs"
)

// getEnhancedFormat adds helpful extensions to certain formats.
func getEnhancedFormat(format string, isInput bool) string {
	f := strings.ToLower(format)
	switch f {
	case MARKDOWN:
		return "markdown+backtick_code_blocks+fenced_code_attributes+footnotes+inline_notes+pipe_tables+raw_html+tex_math_dollars+yaml_metadata_block+auto_identifiers+implicit_header_references"
	case "markdown_strict":
		return "markdown_strict+backtick_code_blocks+pipe_tables"
	case GFM:
		return "gfm+footnotes+tex_math_dollars"
	case HTML:
		return "html+raw_tex+tex_math_dollars"
	case HTML5:
		return "html5+raw_tex+tex_math_dollars"
	case DOCX:
		if isInput {
			return "docx+styles"
		}
		return DOCX
	case LATEX:
		return "latex+raw_html+tex_math_dollars"
	default:
		return f
	}
}
