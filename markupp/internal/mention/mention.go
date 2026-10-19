// Package mention acha menções não ligadas: títulos de notas citados no texto de
// outra nota fora de marcação de ligação, inteiros, com fronteira de palavra, sem
// caixa e sem acento.
package mention

import (
	"iter"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// minLetters é o mínimo de letras de um título para valer como menção.
const minLetters = 3

// linkMarkup casa wikilink, embed e link markdown inteiros, cujo texto já é ligação.
var linkMarkup = regexp.MustCompile(`\[\[[^\]]*\]\]|\[[^\]]*\]\([^)]*\)`)

// Fold é a forma usada para casar menções: sem acento, sem caixa e em NFC.
func Fold(s string) string {
	unaccent := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	stripped, _, err := transform.String(unaccent, s)
	if err != nil {
		stripped = s
	}
	return norm.NFC.String(cases.Fold().String(stripped))
}

// Eligible diz se o título tem letras bastantes para valer como menção.
func Eligible(title string) bool {
	letters := 0
	for _, r := range title {
		if unicode.IsLetter(r) {
			letters++
		}
	}
	return letters >= minLetters
}

// Prior é a probabilidade a priori de a âncora apontar para a nota, com suavização
// de Laplace: (ligações da âncora para a nota + 1) / (ocorrências da âncora +
// número de notas candidatas com ela).
func Prior(anchorLinks, occurrences, candidates int) float64 {
	return float64(anchorLinks+1) / float64(occurrences+candidates)
}

// Matcher acha vários títulos de uma vez no texto, indexados pela primeira palavra
// da forma Fold, como um Aho-Corasick de uma palavra por passo.
type Matcher struct {
	byFirstWord map[string][]foldedTitle
}

type foldedTitle struct {
	folded   string
	original string
}

// NewMatcher prepara os títulos; os que não começam por letra ou dígito nunca casam.
func NewMatcher(titles []string) *Matcher {
	m := &Matcher{byFirstWord: make(map[string][]foldedTitle, len(titles))}
	for _, title := range titles {
		folded := Fold(title)
		if first := firstWord(folded); first != "" && strings.HasPrefix(folded, first) {
			m.byFirstWord[first] = append(m.byFirstWord[first], foldedTitle{folded: folded, original: title})
		}
	}
	return m
}

// Find devolve, uma vez cada e como recebidos, os títulos citados em text fora de
// marcação de ligação.
func (m *Matcher) Find(text string) []string {
	folded := Fold(linkMarkup.ReplaceAllString(text, " "))
	var found []string
	seen := make(map[string]bool)
	for start := range wordStarts(folded) {
		for _, t := range m.byFirstWord[firstWord(folded[start:])] {
			if !seen[t.original] && matchesAt(folded, start, t.folded) {
				seen[t.original] = true
				found = append(found, t.original)
			}
		}
	}
	return found
}

// matchesAt diz se title aparece em text a partir de start e termina em fronteira
// de palavra.
func matchesAt(text string, start int, title string) bool {
	if !strings.HasPrefix(text[start:], title) {
		return false
	}
	next, _ := utf8.DecodeRuneInString(text[start+len(title):])
	return next == utf8.RuneError || !isWordRune(next)
}

// wordStarts produz o índice em bytes de cada início de palavra de text.
func wordStarts(text string) iter.Seq[int] {
	return func(yield func(int) bool) {
		inWord := false
		for i, r := range text {
			starts := isWordRune(r) && !inWord
			inWord = isWordRune(r)
			if starts && !yield(i) {
				return
			}
		}
	}
}

// firstWord é a primeira sequência de letras e dígitos de s.
func firstWord(s string) string {
	start := strings.IndexFunc(s, isWordRune)
	if start < 0 {
		return ""
	}
	end := strings.IndexFunc(s[start:], func(r rune) bool { return !isWordRune(r) })
	if end < 0 {
		return s[start:]
	}
	return s[start : start+end]
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
