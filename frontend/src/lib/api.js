const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || "http://localhost:8080";

export async function apiRequest(
  path,
  { method = "GET", token, body, signal } = {},
) {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  const url = `${API_BASE_URL}${normalizedPath}`;
  const headers = {};

  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
  }

  const response = await fetch(url, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
    signal,
  });

  const raw = await response.text();
  let data = null;

  if (raw) {
    try {
      data = JSON.parse(raw);
    } catch {
      data = { message: raw };
    }
  }

  if (!response.ok) {
    const error = new Error(extractErrorMessage(data) || "Request failed");
    error.status = response.status;
    error.data = data;
    throw error;
  }

  return data;
}

export function extractErrorMessage(errorLike) {
  if (!errorLike) return "Unknown error";
  if (typeof errorLike === "string") return errorLike;
  if (errorLike.message && typeof errorLike.message === "string") return errorLike.message;
  if (errorLike.error && typeof errorLike.error === "string") return errorLike.error;
  return "Request failed";
}
