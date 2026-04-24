/**
 * Extract a scannable snippet from an abstract.
 *
 * PubMed structured abstracts use labels like "BACKGROUND: ", "OBJECTIVE: ",
 * "METHODS: ", etc. For a skim, the OBJECTIVE or BACKGROUND section is usually
 * the most informative — it tells the reader what the paper is trying to do.
 * Falls back to first-N-chars for unstructured abstracts.
 */
export function abstractSnippet(text: string | null | undefined, maxLength = 150): string {
  if (!text) return "";

  // Prefer OBJECTIVE, then BACKGROUND, then the first section present.
  const preferred = findSection(text, ["OBJECTIVE", "OBJECTIVES", "BACKGROUND", "AIM", "AIMS"]);
  const source = preferred ?? text;

  // Trim to maxLength at a word boundary, append ellipsis.
  if (source.length <= maxLength) return source.trim();
  const sliced = source.slice(0, maxLength);
  const lastSpace = sliced.lastIndexOf(" ");
  const cut = lastSpace > 80 ? sliced.slice(0, lastSpace) : sliced;
  return cut.trimEnd() + "…";
}

function findSection(text: string, labels: string[]): string | null {
  for (const label of labels) {
    // Match: start of line OR previous-section separator, then LABEL:
    const re = new RegExp(`(?:^|\\n\\n?)${label}S?:\\s*`);
    const match = text.match(re);
    if (match && match.index !== undefined) {
      const start = match.index + match[0].length;
      // Stop at next labeled section (e.g., "METHODS:") or end of string.
      const after = text.slice(start);
      const next = after.search(/\n\n?[A-Z][A-Z ]+:\s/);
      return (next === -1 ? after : after.slice(0, next)).trim();
    }
  }
  return null;
}
