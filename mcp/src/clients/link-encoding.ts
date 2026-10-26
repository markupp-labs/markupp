// DeeplinkEncoding é como o json de um deeplink entra na url: em base64 (Cursor) ou
// direto, codificado como componente de url (VS Code).
export type DeeplinkEncoding = "base64-json" | "uri-json";

// JsonValue é o que cabe no json de um deeplink.
export type JsonValue =
  string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue };

// strictUriEncode codifica tudo fora de letras, dígitos e -_.~, como o ERB::Util.url_encode
// do Ruby: o painel monta o mesmo deeplink byte a byte.
export function strictUriEncode(text: string): string {
  return encodeURIComponent(text).replace(
    /[!'()*]/g,
    (char) => `%${char.charCodeAt(0).toString(16).toUpperCase()}`,
  );
}

// encodeDeeplinkPayload serializa o json e o codifica para o parâmetro do deeplink.
export function encodeDeeplinkPayload(
  payload: JsonValue,
  encoding: DeeplinkEncoding,
): string {
  const json = JSON.stringify(payload);
  if (encoding === "uri-json") return strictUriEncode(json);
  return strictUriEncode(Buffer.from(json, "utf8").toString("base64"));
}
