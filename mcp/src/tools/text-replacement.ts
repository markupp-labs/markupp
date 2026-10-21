import { UserError } from "fastmcp";

// countOccurrences conta também as ocorrências sobrepostas: "aa" ocorre duas vezes em
// "aaa", e as duas tornam o trecho ambíguo.
export function countOccurrences(content: string, text: string): number {
  let count = 0;
  for (
    let at = content.indexOf(text);
    at !== -1;
    at = content.indexOf(text, at + 1)
  ) {
    count += 1;
  }
  return count;
}

// replaceExactlyOnce troca oldText por newText quando oldText ocorre uma vez só. Fatia
// em vez de String.replace, que interpretaria $& e $1 dentro de newText.
export function replaceExactlyOnce(
  content: string,
  oldText: string,
  newText: string,
): string {
  const count = countOccurrences(content, oldText);
  if (count === 0) {
    throw new UserError(
      "old_text_not_found: old_text occurs 0 times in the note. Call read_note and copy old_text verbatim from the current content, including whitespace and line breaks.",
    );
  }
  if (count > 1) {
    throw new UserError(
      `old_text_ambiguous: old_text occurs ${count} times in the note. Retry with a longer old_text, including surrounding lines, so it occurs exactly once.`,
    );
  }
  const at = content.indexOf(oldText);
  return content.slice(0, at) + newText + content.slice(at + oldText.length);
}
