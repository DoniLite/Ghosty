// =====================
// example_test.go (documentation example)
// =====================
package pandocconv

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func Example() {
	c := New(nil)
	_, _ = c.CreateDefaultCSS()
	html, err := c.Convert(context.Background(), ConvertOptions{
		Content:            "# Mon Document\n\nCeci est un **exemple** avec du *style*.\n\n| Colonne 1 | Colonne 2 |\n|-----------|-----------|\n| Valeur 1  | Valeur 2  |",
		From:               MARKDOWN,
		To:                 HTML,
		PreserveFormatting: true,
		TableOfContents:    true,
		NumberSections:     true,
		Metadata:           map[string]string{"title": "Mon Document", "author": "Votre Nom"},
		Timeout:            15 * time.Second,
	})
	if err != nil { fmt.Println("error:", err); return }
	fmt.Println(strings.HasPrefix(html, "<"))
	// Output: true
}
