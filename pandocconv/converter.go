// Package pandocconv provides a high-level, testable wrapper around the Pandoc CLI
// with sensible defaults for format preservation, assets, metadata, and batch ops.
package pandocconv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// PandocConversionError represents a rich error from the Pandoc CLI execution.
// It mirrors your TS error with stdout/stderr/exit code attached for debugging.
type PandocConversionError struct {
	Msg      string
	Stdout   string
	Stderr   string
	ExitCode int
}

func (e *PandocConversionError) Error() string {
	if e == nil {
		return "<nil>"
	}
	// Keep it short but informative.
	return fmt.Sprintf("pandoc failed (code=%d): %s", e.ExitCode, e.Msg)
}

// Options configures a Converter instance.
type Options struct {
	// Path to pandoc executable; defaults to "pandoc" in PATH.
	PandocPath string
	// Optional working directory for temp files; if empty, os.MkdirTemp is used.
	TempDir string
	// Optional assets used depending on output target.
	CSSPath       string // for html/html5
	LatexTemplate string // for pdf/latex
	ReferenceDoc  string // for docx
}

// ConvertOptions matches the TS options with a few Go-isms (Timeout & ctx).
type ConvertOptions struct {
	Content string
	From    string // see formats.go constants
	To      string // see formats.go constants

	OutputFilePath     string
	AdditionalArgs     []string
	Verbose            bool
	PreserveFormatting bool
	TableOfContents    bool
	NumberSections     bool
	DPI                int
	Metadata           map[string]string

	// Optional timeout; if zero, a sane default is used (e.g., 60s).
	Timeout time.Duration
}

// Converter is the main type for performing conversions.
type Converter struct {
	pandocPath    string
	tempDir       string
	cssPath       string
	latexTemplate string
	referenceDoc  string
}

// ErrDependencyMissing is returned when a dependency is not found in PATH.
var ErrDependencyMissing = errors.New("missing dependency")

// New creates a new Converter with the provided options.
func New(opts *Options) *Converter {
	c := &Converter{
		pandocPath: "pandoc",
	}
	if opts != nil {
		if opts.PandocPath != "" {
			c.pandocPath = opts.PandocPath
		}
		c.tempDir = opts.TempDir
		c.cssPath = opts.CSSPath
		c.latexTemplate = opts.LatexTemplate
		c.referenceDoc = opts.ReferenceDoc
	}
	return c
}

// initTempDir ensures we have a temp directory to work in.
func (c *Converter) initTempDir() (string, error) {
	if c.tempDir != "" {
		return c.tempDir, nil
	}
	d, err := os.MkdirTemp("", "pandoc-converter-")
	if err != nil {
		return "", err
	}
	c.tempDir = d
	return d, nil
}

// buildEnhancedArgs composes pandoc arguments based on ConvertOptions.
func (c *Converter) buildEnhancedArgs(opts ConvertOptions) []string {
	args := []string{}

	fromFmt := getEnhancedFormat(opts.From, true)
	toFmt := getEnhancedFormat(opts.To, false)
	args = append(args, "--from="+fromFmt)
	args = append(args, "--to="+toFmt)

	if opts.PreserveFormatting {
		args = append(args, "--wrap=preserve", "--preserve-tabs")

		switch strings.ToLower(opts.To) {
		case HTML, HTML5:
			args = append(args, "--mathml", "--embed-resources", "--standalone")
		case DOCX:
			args = append(args, "--standalone")
			if c.referenceDoc != "" {
				args = append(args, "--reference-doc="+c.referenceDoc)
			}
		case PDF, LATEX:
			args = append(args, "--standalone")
			if c.latexTemplate != "" {
				args = append(args, "--template="+c.latexTemplate)
			}
		}
	}

	if opts.TableOfContents {
		args = append(args, "--toc", "--toc-depth=3")
	}
	if opts.NumberSections {
		args = append(args, "--number-sections")
	}
	if opts.DPI > 0 {
		args = append(args, fmt.Sprintf("--dpi=%d", opts.DPI))
	}

	if (strings.EqualFold(opts.To, HTML) || strings.EqualFold(opts.To, HTML5)) && c.cssPath != "" {
		args = append(args, "--css="+c.cssPath)
	}

	for k, v := range opts.Metadata {
		args = append(args, fmt.Sprintf("--metadata=%s:%s", k, v))
	}

	if opts.OutputFilePath != "" {
		args = append(args, "--output="+opts.OutputFilePath)
	}

	if len(opts.AdditionalArgs) > 0 {
		args = append(args, opts.AdditionalArgs...)
	}

	return args
}

// Convert runs pandoc with the provided options. If OutputFilePath is empty, returns stdout.
func (c *Converter) Convert(ctx context.Context, opts ConvertOptions) (string, error) {
	if _, err := c.initTempDir(); err != nil {
		return "", err
	}
	cmdCtx := ctx
	if cmdCtx == nil {
		cmdCtx = context.Background()
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 60 * time.Second
	}
	var cancel context.CancelFunc
	cmdCtx, cancel = context.WithTimeout(cmdCtx, opts.Timeout)
	defer cancel()

	args := c.buildEnhancedArgs(opts)

	cmd := exec.CommandContext(cmdCtx, c.pandocPath, args...)
	// We'll feed content on stdin and collect stdout/stderr.
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	if opts.OutputFilePath == "" {
		cmd.Stdout = &stdoutBuf
	} else {
		cmd.Stdout = io.Discard // pandoc writes file itself
	}
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return "", &PandocConversionError{Msg: "failed to start pandoc", Stderr: err.Error(), ExitCode: -1}
	}

	// Write content to stdin and close.
	_, _ = io.WriteString(stdin, opts.Content)
	_ = stdin.Close()

	err = cmd.Wait()
	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else if cmdCtx.Err() == context.DeadlineExceeded {
			exitCode = -2
			stderrStr = stderrStr + "\ncontext deadline exceeded"
		} else {
			exitCode = -1
		}
		return "", &PandocConversionError{Msg: "pandoc conversion failed", Stdout: stdoutStr, Stderr: stderrStr, ExitCode: exitCode}
	}

	if opts.OutputFilePath != "" {
		return opts.OutputFilePath, nil
	}
	return stdoutStr, nil
}

// CreateTempFile writes content to a file under the converter's temp dir.
func (c *Converter) CreateTempFile(content, filename string) (string, error) {
	if _, err := c.initTempDir(); err != nil {
		return "", err
	}
	p := filepath.Join(c.tempDir, filename)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		return "", err
	}
	return p, nil
}

// CreateDefaultCSS generates a clean CSS and stores its path for subsequent HTML outputs.
func (c *Converter) CreateDefaultCSS() (string, error) {
	css := `/* Enhanced document styling for better compatibility */
body { font-family: -apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif; line-height:1.6; color:#333; max-width:800px; margin:0 auto; padding:20px; background:#fff; }
h1,h2,h3,h4,h5,h6 { color:#2c3e50; margin-top:1.5em; margin-bottom:0.5em; font-weight:600; }
h1{font-size:2.25em; border-bottom:2px solid #3498db; padding-bottom:10px;} h2{font-size:1.75em; border-bottom:1px solid #ecf0f1; padding-bottom:5px;} h3{font-size:1.5em;} h4{font-size:1.25em;}
p{margin-bottom:1em;}
/* Tables */
table{border-collapse:collapse; width:100%; margin:1em 0;}
th,td{border:1px solid #ddd; padding:12px; text-align:left;}
th{background:#f8f9fa; font-weight:600;} tr:nth-child(even){background:#f8f9fa;}
/* Code */
pre{background:#f8f9fa; border:1px solid #e9ecef; border-radius:4px; padding:1em; overflow-x:auto; font-size:.9em;}
code{background:#f8f9fa; padding:2px 4px; border-radius:3px; font-size:.9em;}
/* Lists */
ul,ol{margin-bottom:1em; padding-left:2em;} li{margin-bottom:.5em;}
/* Blockquotes */
blockquote{border-left:4px solid #3498db; margin:1em 0; padding-left:1em; color:#555; font-style:italic;}
/* Images */
img{max-width:100%; height:auto; display:block; margin:1em auto;}
/* Links */
a{color:#3498db; text-decoration:none;} a:hover{text-decoration:underline;}
/* Print */
@media print{ body{max-width:none; padding:0;} h1,h2,h3,h4,h5,h6{page-break-after:avoid;} pre,blockquote,table{page-break-inside:avoid;} }
`
	p, err := c.CreateTempFile(css, "default-styles.css")
	if err != nil {
		return "", err
	}
	c.cssPath = p
	return p, nil
}

// CleanupTempDir removes the converter's temp directory, if any.
func (c *Converter) CleanupTempDir() error {
	if c.tempDir == "" {
		return nil
	}
	return os.RemoveAll(c.tempDir)
}

// Dependencies reports availability of common tools used in extended conversions.
type Dependencies struct {
	Pandoc          bool
	LaTeX           bool // pdflatex
	ImageMagick     bool // convert
	LibreOffice     bool // libreoffice or soffice
	Wkhtmltopdf     bool
	Missing         []string
	Recommendations []string
}

// CheckDependencies probes the system for required/optional tools.
func (c *Converter) CheckDependencies(ctx context.Context) Dependencies {
	check := func(names ...string) bool {
		for _, n := range names {
			if p, _ := exec.LookPath(n); p != "" {
				return true
			}
		}
		return false
	}
	deps := Dependencies{}
	// Pandoc path may be custom.
	if p, _ := exec.LookPath(c.pandocPath); p != "" {
		deps.Pandoc = true
	}
	deps.LaTeX = check("pdflatex")
	deps.ImageMagick = check("convert")
	deps.LibreOffice = check("libreoffice", "soffice")
	deps.Wkhtmltopdf = check("wkhtmltopdf")

	if !deps.Pandoc {
		deps.Missing = append(deps.Missing, "pandoc")
	}
	if !deps.LaTeX {
		deps.Missing = append(deps.Missing, "texlive/mactex/miktex")
		deps.Recommendations = append(deps.Recommendations, "Install LaTeX for PDF conversion support")
	}
	if !deps.ImageMagick {
		deps.Missing = append(deps.Missing, "imagemagick")
		deps.Recommendations = append(deps.Recommendations, "Install ImageMagick for better image handling")
	}
	if !deps.LibreOffice {
		deps.Missing = append(deps.Missing, "libreoffice")
		deps.Recommendations = append(deps.Recommendations, "Install LibreOffice for ODT/DOC support")
	}
	if !deps.Wkhtmltopdf {
		deps.Missing = append(deps.Missing, "wkhtmltopdf")
		deps.Recommendations = append(deps.Recommendations, "Install wkhtmltopdf for HTML→PDF conversion (wkhtmltopdf)")
	}

	_ = ctx // reserved for future per-tool --version checks
	return deps
}

// ConvertBatch performs multiple conversions with shared settings.
func (c *Converter) ConvertBatch(ctx context.Context, docs []struct {
	Content  string
	Filename string
	From     string
	To       string
}, base PartialConvertOptions) ([]struct {
	Filename string
	Content  string
	Success  bool
	Error    string
}, error) {
	res := make([]struct {
		Filename string
		Content  string
		Success  bool
		Error    string
	}, 0, len(docs))

	for _, d := range docs {
		out, err := c.Convert(ctx, ConvertOptions{
			Content:            d.Content,
			From:               d.From,
			To:                 d.To,
			OutputFilePath:     base.OutputFilePath,
			AdditionalArgs:     base.AdditionalArgs,
			Verbose:            base.Verbose,
			PreserveFormatting: base.PreserveFormatting,
			TableOfContents:    base.TableOfContents,
			NumberSections:     base.NumberSections,
			DPI:                base.DPI,
			Metadata:           base.Metadata,
			Timeout:            base.Timeout,
		})
		item := struct {
			Filename string
			Content  string
			Success  bool
			Error    string
		}{Filename: d.Filename}
		if err != nil {
			item.Success = false
			item.Error = err.Error()
		} else {
			item.Success = true
			item.Content = out
		}
		res = append(res, item)
	}
	return res, nil
}

// PartialConvertOptions mirrors your TS Partial<ConvertOptions> for batch defaults.
type PartialConvertOptions struct {
	OutputFilePath     string
	AdditionalArgs     []string
	Verbose            bool
	PreserveFormatting bool
	TableOfContents    bool
	NumberSections     bool
	DPI                int
	Metadata           map[string]string
	Timeout            time.Duration
}

// OSHint returns a brief hint string about platform-specific PDF toolchains.
func OSHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS: prefer MacTeX for LaTeX; wkhtmltopdf via Homebrew."
	case "windows":
		return "Windows: use MiKTeX/TeX Live and wkhtmltopdf; ensure pandoc is in PATH."
	default:
		return "Linux: install texlive-full (or minimal + required packages), imagemagick, libreoffice, wkhtmltopdf."
	}
}

// CheckDependencies ensures required executables are present.
func CheckDependencies() error {
	deps := []string{"pandoc"}
	for _, dep := range deps {
		if _, err := exec.LookPath(dep); err != nil {
			return fmt.Errorf("%w: %s", ErrDependencyMissing, dep)
		}
	}
	return nil
}
