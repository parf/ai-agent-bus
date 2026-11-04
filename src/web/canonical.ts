export function canonicalInput(query: URLSearchParams, form?: URLSearchParams): void {
  for (const [key] of query) {
    if (key.toLowerCase() === "personal") throw new Error("Use tag=personal; the personal query field is not supported.");
  }
  for (const [key] of form ?? []) {
    if (["personal", "edit_personal", "from_personal"].includes(key.toLowerCase()))
      throw new Error("Use tags for Personal classification; the submitted field is not supported.");
  }
}
