export function parseJwt(token) {
  if (!token) return null;
  const chunks = token.split(".");
  if (chunks.length !== 3) return null;

  try {
    const payload = chunks[1]
      .replace(/-/g, "+")
      .replace(/_/g, "/")
      .padEnd(Math.ceil(chunks[1].length / 4) * 4, "=");
    return JSON.parse(atob(payload));
  } catch {
    return null;
  }
}
