// O ts_headline da busca marca cada termo achado entre os caracteres de controle
// \u0002 e \u0003; o agente lê markdown, então eles viram negrito.
const highlightMarkers = /[\u0002\u0003]/g;

// markdownHighlight troca os marcadores de realce do trecho por **negrito**.
export function markdownHighlight(snippet: string): string {
  return snippet.replace(highlightMarkers, "**");
}
