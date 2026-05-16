const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || "http://localhost:8080";

export async function apiGet(path, token) {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    headers: authHeaders(token),
  });
  return parseResponse(response);
}

export async function apiPost(path, token, body) {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...authHeaders(token),
    },
    body: JSON.stringify(body),
  });
  return parseResponse(response);
}

export async function apiPatch(path, token, body) {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: "PATCH",
    headers: {
      "Content-Type": "application/json",
      ...authHeaders(token),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  return parseResponse(response);
}

export async function apiDelete(path, token) {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: "DELETE",
    headers: authHeaders(token),
  });
  return parseResponse(response);
}

export async function apiMultipart(path, token, body) {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: "POST",
    headers: authHeaders(token),
    body,
  });
  return parseResponse(response);
}

export async function apiMultipartPatch(path, token, body) {
  const response = await fetch(`${API_BASE_URL}${path}`, {
    method: "PATCH",
    headers: authHeaders(token),
    body,
  });
  return parseResponse(response);
}

export function wsURL(path, token) {
  const base = new URL(API_BASE_URL);
  base.protocol = base.protocol === "https:" ? "wss:" : "ws:";
  base.pathname = path;
  base.search = token ? `access_token=${encodeURIComponent(token)}` : "";
  return base.toString();
}

export function apiURL(path) {
  if (/^https?:\/\//i.test(path)) {
    return path;
  }
  return `${API_BASE_URL}${path}`;
}

function authHeaders(token) {
  return token ? { Authorization: `Bearer ${token}` } : {};
}

async function parseResponse(response) {
  const text = await response.text();
  const data = text ? JSON.parse(text) : null;
  if (!response.ok) {
    if (response.status === 401) {
      window.dispatchEvent(new CustomEvent("wm-auth-expired"));
    }
    throw new Error(data?.error || `Request failed: ${response.status}`);
  }
  return data;
}
