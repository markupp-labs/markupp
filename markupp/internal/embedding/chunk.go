package embedding

import "unicode/utf8"

// maxChunks é quantos pedaços de uma nota viram vetor; o resto do texto fica de fora.
const maxChunks = 8

// charsPerToken converte tokens em letras para o provedor remoto, que não expõe o
// tokenizador dele.
const charsPerToken = 4

// Chunks corta text em pedaços de até chunkTokens tokens, nas fronteiras de spans (o
// início e o fim, em bytes, de cada token), e fica com os maxChunks primeiros.
func Chunks(text string, spans [][2]int, chunkTokens int) []string {
	var chunks []string
	for start := 0; start < len(spans) && len(chunks) < maxChunks; start += chunkTokens {
		end := min(start+chunkTokens, len(spans)) - 1
		chunks = append(chunks, text[spans[start][0]:spans[end][1]])
	}
	return chunks
}

// CharSpans marca um token a cada charsPerToken letras de text.
func CharSpans(text string) [][2]int {
	spans := make([][2]int, 0, utf8.RuneCountInString(text)/charsPerToken+1)
	start, letters := 0, 0
	for i := range text {
		if letters == charsPerToken {
			spans = append(spans, [2]int{start, i})
			start, letters = i, 0
		}
		letters++
	}
	if letters > 0 {
		spans = append(spans, [2]int{start, len(text)})
	}
	return spans
}
