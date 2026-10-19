// Bloco cercado na leitura: os números de linha numa coluna própria antes do code, aria-hidden e fora da seleção
// (user-select no CSS), então copiar o bloco copia só o código.
export function withLineNumbers(renderFence) {
  return (tokens, index, ...rest) => {
    const count = tokens[index].content.replace(/\n$/, "").split("\n").length
    const numbers = Array.from({ length: count }, (_, line) => line + 1).join("\n")
    return renderFence(tokens, index, ...rest).replace("<code", `<span class="code-lines" aria-hidden="true">${numbers}</span><code`)
  }
}

// Cabeçalho de cada bloco (o <template> codeTools do form: a linguagem, o botão de copiar e o status que o leitor de
// tela lê), no topo do pre. Entra depois do DOMPurify, que não deixaria passar um botão vindo do markdown.
export function addCodeTools(root, template) {
  root.querySelectorAll("pre:has(> code)").forEach((pre) => {
    const header = template.content.cloneNode(true)
    header.querySelector(".code-language").textContent = codeLanguage(pre.querySelector(":scope > code"))
    pre.prepend(header)
  })
}

// Linguagem da cerca (class language-x do markdown-it); sem ela, ou no bloco indentado, "text".
const codeLanguage = (code) => code.className.match(/\blanguage-(\S+)/)?.[1] ?? "text"

// Copia só o texto do code (os números moram fora dele) e diz o resultado no status do bloco por 2 segundos.
export async function copyCode(button, { copied, failed }) {
  const pre = button.closest("pre")
  const status = pre.querySelector(".code-header [role=status]")
  try {
    await navigator.clipboard.writeText(pre.querySelector(":scope > code").textContent)
    status.textContent = copied
  } catch {
    status.textContent = failed
  }
  clearTimeout(Number(status.dataset.timer))
  status.dataset.timer = setTimeout(() => (status.textContent = ""), 2000)
}
