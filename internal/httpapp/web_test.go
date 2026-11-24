package httpapp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// As telas antigas não podem ganhar script inline nem origem externa fora da CSP sem que a
// CSP acompanhe: o navegador bloquearia em silêncio.
func TestTelasAntigasCabemNaCSP(t *testing.T) {
	paginas, err := filepath.Glob(filepath.Join("..", "..", "apps", "*", "web", "okf", "*.html"))
	if err != nil || len(paginas) == 0 {
		t.Fatalf("páginas de web/okf: %v", err)
	}
	scriptInline := regexp.MustCompile(`<script(\s[^>]*)?>\s*[^<\s]`)
	origemExterna := regexp.MustCompile(`(?:src|href)="(https://[^/"]+)`)
	manipuladorInline := regexp.MustCompile(`\son[a-z]+="`)
	for _, p := range paginas {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		html := string(raw)
		if scriptInline.MatchString(html) || manipuladorInline.MatchString(html) {
			t.Errorf("%s: script inline exige 'unsafe-inline' em script-src", p)
		}
		for _, m := range origemExterna.FindAllStringSubmatch(html, -1) {
			if !strings.Contains(cspLegado, m[1]) {
				t.Errorf("%s: %s fora da CSP das telas antigas", p, m[1])
			}
		}
	}
}
