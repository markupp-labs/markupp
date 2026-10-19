// Bloco cercado na leitura: os números de linha numa coluna própria antes do code, aria-hidden e fora da seleção
// (user-select no CSS), então copiar o bloco copia só o código.
export function withLineNumbers(renderFence) {
  return (tokens, index, ...rest) => {
    const count = tokens[index].content.replace(/\n$/, "").split("\n").length
    const numbers = Array.from({ length: count }, (_, line) => line + 1).join("\n")
    return renderFence(tokens, index, ...rest).replace("<code", `<span class="code-lines" aria-hidden="true">${numbers}</span><code`)
  }
}
