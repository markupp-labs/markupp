// Troca parâmetros da query de uma URL (objeto URL, alterado no lugar): valor null tira o parâmetro.
export function replaceQuery(url, query) {
  for (const [ name, value ] of Object.entries(query)) value == null ? url.searchParams.delete(name) : url.searchParams.set(name, value)
  return url
}
