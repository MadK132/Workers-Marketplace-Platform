import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { apiDelete, apiGet, apiMultipart, apiMultipartPatch, apiPatch, apiPost, apiURL, wsURL } from "./api.js";
import MapView from "./MapView.jsx";
import WorkerList from "./WorkerList.jsx";
import { useGeolocation } from "./useGeolocation.js";

const TOKEN_KEY = "workers_marketplace_token";
const ROLE_KEY = "workers_marketplace_role";
const PAYMENT_SETUP_INTENT_KEY = "workers_marketplace_payment_setup_intent";
const ASTANA_BOUNDS = {
  minLatitude: 50.95,
  maxLatitude: 51.35,
  minLongitude: 71.15,
  maxLongitude: 71.75,
};
const GIS_API_KEY = import.meta.env.VITE_2GIS_API_KEY || "";

const roleTabs = {
  customer: [
    ["find", "Search"],
    ["requests", "Requests"],
    ["bookings", "Bookings"],
    ["chats", "Chat"],
    ["profile", "Profile"],
    ["notifications", "Alerts"],
  ],
  worker: [
    ["pro", "Map"],
    ["jobs", "Jobs"],
    ["chats", "Chat"],
    ["skills", "Services"],
    ["profile", "Profile"],
    ["notifications", "Alerts"],
  ],
  admin: [
    ["overview", "Dashboard"],
    ["verify", "Queue"],
    ["users", "Users"],
    ["accounts", "Staff"],
    ["notifications", "Alerts"],
  ],
  manager: [
    ["overview", "Dashboard"],
    ["verify", "Queue"],
    ["users", "Users"],
    ["notifications", "Alerts"],
  ],
};

export default function App() {
  const [token, setToken] = useState(() => localStorage.getItem(TOKEN_KEY) || "");
  const [role, setRole] = useState(() => localStorage.getItem(ROLE_KEY) || readRole(token));
  const [activeTab, setActiveTab] = useState(defaultTabForRole(role));
  const [paymentReady, setPaymentReady] = useState(false);
  const [paymentLoading, setPaymentLoading] = useState(false);
  const [paymentError, setPaymentError] = useState("");
  const session = useMemo(() => decodeToken(token), [token]);
  const { toastNotifications, dismissToastNotification } = useNotificationFeed(token);

  const openToastAction = useCallback(async (item) => {
    if (!token) {
      return;
    }
    if (item.action_type === "booking_chat" && item.action_ref) {
      const chat = await apiPost("/api/chats", token, { booking_id: Number(item.action_ref) });
      if (chat?.chat_id) {
        localStorage.setItem("workers_marketplace_active_chat", String(chat.chat_id));
      }
      setActiveTab("chats");
    } else if (item.action_type === "chat" && item.action_ref) {
      localStorage.setItem("workers_marketplace_active_chat", String(item.action_ref));
      setActiveTab("chats");
    }
    const id = item.notification_id || item.id;
    if (id) {
      await apiPatch(`/api/notifications/${id}/read`, token, {}).catch(() => {});
      dismissToastNotification(notificationID(item));
      window.dispatchEvent(new CustomEvent("wm-notifications-updated"));
    }
  }, [dismissToastNotification, token]);

  const saveSession = useCallback((nextToken, fallbackRole) => {
    const nextRole = readRole(nextToken) || fallbackRole || "";
    localStorage.setItem(TOKEN_KEY, nextToken);
    localStorage.setItem(ROLE_KEY, nextRole);
    setToken(nextToken);
    setRole(nextRole);
    setActiveTab(defaultTabForRole(nextRole));
    setPaymentReady(false);
  }, []);

  const signOut = useCallback(async (options = {}) => {
    const disableWorker = options.disableWorker !== false;
    const currentToken = localStorage.getItem(TOKEN_KEY) || token;
    const currentRole = localStorage.getItem(ROLE_KEY) || role;
    if (disableWorker && currentRole === "worker" && currentToken) {
      try {
        await fetch(apiURL("/api/worker/availability"), {
          method: "PATCH",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${currentToken}`,
          },
          body: JSON.stringify({ is_available: false }),
        });
      } catch {
        // Token may already be expired; local logout must still complete.
      }
    }
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(ROLE_KEY);
    clearAuthURL();
    setToken("");
    setRole("");
    setActiveTab("find");
    setPaymentReady(false);
  }, [role, token]);

  useEffect(() => {
    if (!token || (role !== "customer" && role !== "worker")) {
      setPaymentReady(role !== "customer" && role !== "worker");
      setPaymentError("");
      setPaymentLoading(false);
      return undefined;
    }
    let cancelled = false;
    setPaymentLoading(true);
    setPaymentError("");
    apiGet("/api/payment-method", token)
      .then((method) => {
        if (!cancelled) {
          setPaymentReady(Boolean(method?.has_payment_method));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setPaymentReady(false);
          setPaymentError(err.message);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setPaymentLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [role, token]);

  useEffect(() => {
    if (!token || !window.location.pathname.toLowerCase().includes("/payment/setup-success")) {
      return;
    }
    const sessionID = new URLSearchParams(window.location.search).get("session_id");
    if (!sessionID) {
      return;
    }
    apiPost("/api/payment-method/stripe/confirm", token, { session_id: sessionID })
      .then(() => {
        setPaymentReady(true);
        clearAuthURL();
        window.dispatchEvent(new CustomEvent("wm-payment-method-linked"));
      })
      .catch((err) => setPaymentError(err.message));
  }, [token]);

  useEffect(() => {
    if (!token) {
      return undefined;
    }
    const expiresAt = Number(session.exp || 0) * 1000;
    if (!expiresAt) {
      return undefined;
    }
    const now = Date.now();
    if (expiresAt <= now) {
      signOut();
      return undefined;
    }
    const disableDelay = Math.max(0, expiresAt - now - 10000);
    const logoutDelay = Math.max(0, expiresAt - now + 250);
    const disableTimer = window.setTimeout(() => {
      if ((localStorage.getItem(ROLE_KEY) || role) === "worker") {
        apiPatch("/api/worker/availability", localStorage.getItem(TOKEN_KEY) || token, { is_available: false }).catch(() => {});
      }
    }, disableDelay);
    const logoutTimer = window.setTimeout(() => {
      signOut({ disableWorker: false });
    }, logoutDelay);
    return () => {
      window.clearTimeout(disableTimer);
      window.clearTimeout(logoutTimer);
    };
  }, [role, session.exp, signOut, token]);

  useEffect(() => {
    if (!token) {
      return undefined;
    }

    const checkSession = () => {
      const currentToken = localStorage.getItem(TOKEN_KEY) || "";
      if (isTokenExpired(currentToken)) {
        signOut();
      }
    };

    checkSession();
    const intervalID = window.setInterval(checkSession, 30000);
    window.addEventListener("focus", checkSession);
    document.addEventListener("visibilitychange", checkSession);
    window.addEventListener("pageshow", checkSession);

    return () => {
      window.clearInterval(intervalID);
      window.removeEventListener("focus", checkSession);
      document.removeEventListener("visibilitychange", checkSession);
      window.removeEventListener("pageshow", checkSession);
    };
  }, [signOut, token]);

  useEffect(() => {
    const handleExpired = () => {
      signOut();
    };
    window.addEventListener("wm-auth-expired", handleExpired);
    return () => window.removeEventListener("wm-auth-expired", handleExpired);
  }, [signOut]);

  if (!token) {
    return <AuthScreen onAuth={saveSession} />;
  }

  const startPaymentSetup = async () => {
    setPaymentError("");
    const result = await apiPost("/api/payment-method/stripe/setup-session", token, {});
    const setupURL = result?.payment_setup_url || result?.url;
    if (!setupURL) {
      throw new Error("Payment setup URL is missing.");
    }
    window.location.href = setupURL;
  };

  if (role === "customer") {
    return (
      <>
        <main className="workerFullscreen">
          <CustomerApp
            token={token}
            activeTab={activeTab}
            onNavigate={setActiveTab}
            onSignOut={signOut}
            paymentReady={paymentReady}
            paymentLoading={paymentLoading}
            paymentError={paymentError}
            onStartPaymentSetup={startPaymentSetup}
          />
        </main>
        <NotificationToasts items={toastNotifications} onDismiss={dismissToastNotification} onAction={openToastAction} />
      </>
    );
  }

  if (role === "worker") {
    return (
      <>
        <main className="workerFullscreen">
          <WorkerApp
            token={token}
            activeTab={activeTab}
            onNavigate={setActiveTab}
            onSignOut={signOut}
            paymentReady={paymentReady}
            paymentLoading={paymentLoading}
            paymentError={paymentError}
            onStartPaymentSetup={startPaymentSetup}
          />
        </main>
        <NotificationToasts items={toastNotifications} onDismiss={dismissToastNotification} onAction={openToastAction} />
      </>
    );
  }

  return (
    <>
      <AppFrame role={role} session={session} activeTab={activeTab} onTab={setActiveTab} onSignOut={signOut}>
        {(role === "admin" || role === "manager") && <AdminApp token={token} role={role} activeTab={activeTab} onNavigate={setActiveTab} />}
        {!["customer", "worker", "admin", "manager"].includes(role) && (
          <EmptyState title="Role is missing" text="Sign out and sign in again, or select a role in the backend." />
        )}
      </AppFrame>
      <NotificationToasts items={toastNotifications} onDismiss={dismissToastNotification} onAction={openToastAction} />
    </>
  );
}

function AuthScreen({ onAuth }) {
  const [mode, setMode] = useState(() => initialAuthMode());
  const activationStarted = useRef(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [login, setLogin] = useState({ email: "", password: "" });
  const [register, setRegister] = useState({
    full_name: "",
    email: "",
    phone: "",
    password: "",
    role: "customer",
  });
  const [forgotEmail, setForgotEmail] = useState("");
  const [reset, setReset] = useState(() => ({
    token: tokenFromURL(),
    new_password: "",
    confirm_password: "",
  }));
  const [activationToken] = useState(() => tokenFromURL());

  async function run(action) {
    setBusy(true);
    setError("");
    setMessage("");
    try {
      await action();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  const changeMode = useCallback((nextMode) => {
    setMode(nextMode);
    if (nextMode === "signin" || nextMode === "signup" || nextMode === "forgot") {
      clearAuthURL();
    }
  }, []);

  const submitLogin = (event) => {
    event.preventDefault();
    run(async () => {
      const data = await apiPost("/auth/login", "", login);
      const accessToken = data.access_token || data.token;
      if (!accessToken) {
        throw new Error("Login response does not contain access token.");
      }
      onAuth(accessToken);
    });
  };

  const submitRegister = (event) => {
    event.preventDefault();
    run(async () => {
      await apiPost("/auth/register", "", register);
      setMessage("Account created. Check email verification before signing in.");
      setRegister({
        full_name: "",
        email: "",
        phone: "",
        password: "",
        role: "customer",
      });
      clearAuthURL();
      setMode("signin");
    });
  };

  const submitForgot = (event) => {
    event.preventDefault();
    run(async () => {
      await apiPost("/auth/forgot-password", "", { email: forgotEmail });
      setMessage("Password reset email was sent if the account exists.");
      clearAuthURL();
      setMode("signin");
    });
  };

  const submitReset = (event) => {
    event.preventDefault();
    run(async () => {
      if (reset.new_password !== reset.confirm_password) {
        throw new Error("Passwords do not match.");
      }
      await apiPost("/auth/reset-password", "", {
        token: reset.token,
        new_password: reset.new_password,
      });
      setMessage("Password was reset. You can sign in now.");
      clearAuthURL();
      setMode("signin");
    });
  };

  useEffect(() => {
    if (mode !== "activation" || activationStarted.current) {
      return;
    }
    activationStarted.current = true;
    run(async () => {
      if (!activationToken) {
        throw new Error("Activation token is missing.");
      }
      await apiGet(`/auth/verify?token=${encodeURIComponent(activationToken)}`, "");
      setMessage("Email activated. You can sign in now.");
    });
  }, [activationToken, mode]);

  return (
    <main className="authShell">
      <section className="authIntro">
        <div className="appIcon">WM</div>
        <h1>Workers Marketplace</h1>
        <p>Sign in first. Then the app opens either the customer flow for hiring or the worker Pro flow for accepting jobs.</p>
      </section>
      <section className="authCard">
        {(mode === "signin" || mode === "signup") && (
          <div className="authTabs">
            <button className={mode === "signin" ? "active" : ""} onClick={() => changeMode("signin")}>Sign in</button>
            <button className={mode === "signup" ? "active" : ""} onClick={() => changeMode("signup")}>Sign up</button>
          </div>
        )}

        {mode === "signin" && (
          <form className="formStack" onSubmit={submitLogin}>
            <Field label="Email"><input value={login.email} onChange={(e) => setLogin({ ...login, email: e.target.value })} type="email" required /></Field>
            <Field label="Password"><input value={login.password} onChange={(e) => setLogin({ ...login, password: e.target.value })} type="password" required /></Field>
            <button disabled={busy}>Sign in</button>
            <div className="authLinks">
              <button type="button" onClick={() => changeMode("forgot")}>Forgot password?</button>
            </div>
          </form>
        )}

        {mode === "signup" && (
          <form className="formStack" onSubmit={submitRegister}>
            <Field label="Full name"><input value={register.full_name} onChange={(e) => setRegister({ ...register, full_name: e.target.value })} required /></Field>
            <Field label="Email"><input value={register.email} onChange={(e) => setRegister({ ...register, email: e.target.value })} type="email" required /></Field>
            <Field label="Phone"><input value={register.phone} onChange={(e) => setRegister({ ...register, phone: e.target.value })} required /></Field>
            <Field label="Role">
              <select value={register.role} onChange={(e) => setRegister({ ...register, role: e.target.value })}>
                <option value="customer">Customer</option>
                <option value="worker">Worker</option>
              </select>
            </Field>
            <Field label="Password"><input value={register.password} onChange={(e) => setRegister({ ...register, password: e.target.value })} type="password" required /></Field>
            <button disabled={busy}>Create account</button>
          </form>
        )}

        {mode === "forgot" && (
          <form className="formStack" onSubmit={submitForgot}>
            <AuthTitle title="Password reset" text="Enter your account email. We will send a password reset link there." />
            <Field label="Email"><input value={forgotEmail} onChange={(e) => setForgotEmail(e.target.value)} type="email" required /></Field>
            <button disabled={busy}>Send reset email</button>
            <div className="authLinks">
              <button type="button" onClick={() => changeMode("signin")}>Back to sign in</button>
            </div>
          </form>
        )}

        {mode === "reset" && (
          <form className="formStack" onSubmit={submitReset}>
            <AuthTitle title="Set new password" text="This page opens from the password reset link in your email." />
            <Field label="New password"><input value={reset.new_password} onChange={(e) => setReset({ ...reset, new_password: e.target.value })} type="password" required /></Field>
            <Field label="Repeat new password"><input value={reset.confirm_password} onChange={(e) => setReset({ ...reset, confirm_password: e.target.value })} type="password" required /></Field>
            <button disabled={busy}>Set new password</button>
            <div className="authLinks">
              <button type="button" onClick={() => changeMode("signin")}>Back to sign in</button>
            </div>
          </form>
        )}

        {mode === "activation" && (
          <div className="formStack">
            <AuthTitle title="Email activation" text={busy ? "Activating your email..." : "Your email activation link has been processed."} />
            {busy && <div className="loader" />}
            <div className="authLinks">
              <button type="button" onClick={() => changeMode("signin")}>Back to sign in</button>
            </div>
          </div>
        )}

        <Messages message={message} error={error} />
      </section>
    </main>
  );
}

function AuthTitle({ title, text }) {
  return (
    <header className="authTitle">
      <h2>{title}</h2>
      <p>{text}</p>
    </header>
  );
}

function PaymentSetupScreen({ token, role, loading, error, onReady, onSignOut }) {
  return (
    <main className="authLayout">
      <section className="authHero">
        <div className="appIcon">WM</div>
        <h1>Workers Marketplace</h1>
        <p>{role === "worker" ? "Link a card before going online." : "Link a card before booking workers."}</p>
      </section>
      <section className="authCard">
        <AuthTitle title="Payment card required" text="Add your card once, then continue to the marketplace." />
        {loading ? <p className="muted">Checking payment method...</p> : <PaymentMethodPanel token={token} onLinked={onReady} compact />}
        <Messages error={error} />
        <button className="secondaryButton fullWidthButton" onClick={onSignOut}>Sign out</button>
      </section>
    </main>
  );
}

function initialAuthMode() {
  const path = window.location.pathname.toLowerCase();
  const params = new URLSearchParams(window.location.search);
  const mode = params.get("mode")?.toLowerCase();
  if (path.includes("reset") || mode === "reset") {
    return "reset";
  }
  if (path.includes("verify") || path.includes("activate") || mode === "activation" || mode === "verify") {
    return "activation";
  }
  return "signin";
}

function tokenFromURL() {
  const params = new URLSearchParams(window.location.search);
  return params.get("token") || params.get("reset_token") || params.get("verification_token") || "";
}

function isEmptyResultError(err) {
  const message = err?.message?.toLowerCase() || "";
  return message.includes("no rows in result set");
}

function categoryTitle(name) {
  const normalized = String(name || "").toLowerCase();
  const titles = {
    appliance_installation: "Appliance installation",
    carpenter: "Carpenter",
    carpentry: "Carpenter",
    cleaner: "Cleaning",
    cleaning: "Cleaning",
    electrician: "Electrician",
    electrical: "Electrician",
    gardener: "Gardener",
    mover: "Mover",
    plumber: "Plumber",
    plumbing: "Plumber",
    renovation: "Renovation",
    painting: "Painting",
  };
  return titles[normalized] || humanize(name);
}

function categoryDescription(name, fallback) {
  const normalized = String(name || "").toLowerCase();
  const descriptions = {
    appliance_installation: "Appliance setup and home device installation.",
    carpenter: "Furniture assembly, doors and small wood repairs.",
    carpentry: "Furniture assembly, doors and small wood repairs.",
    cleaner: "Apartment, house or office cleaning.",
    cleaning: "Apartment, house or office cleaning.",
    electrician: "Sockets, lighting, wiring and diagnostics.",
    electrical: "Sockets, lighting, wiring and diagnostics.",
    gardener: "Garden and plant care.",
    mover: "Loading, carrying and moving help.",
    plumber: "Pipes, leaks, mixers and plumbing.",
    plumbing: "Pipes, leaks, mixers and plumbing.",
    renovation: "Finishing and renovation work.",
    painting: "Walls, ceilings and decorative painting.",
  };
  return descriptions[normalized] || fallback || "Service category";
}

function experienceTitle(level) {
  const titles = {
    junior: "junior",
    middle: "middle",
    senior: "senior",
  };
  return titles[level] || humanize(level);
}

function humanize(value) {
  return String(value || "")
    .replaceAll("_", " ")
    .replace(/\b\w/g, (letter) => letter.toUpperCase());
}

function isMissingWorkerProfileError(err) {
  const message = err?.message?.toLowerCase() || "";
  return message.includes("no rows in result set") || message.includes("worker profile not found");
}

function isInsideAstana(position) {
  if (!position) {
    return false;
  }
  return position.latitude >= ASTANA_BOUNDS.minLatitude &&
    position.latitude <= ASTANA_BOUNDS.maxLatitude &&
    position.longitude >= ASTANA_BOUNDS.minLongitude &&
    position.longitude <= ASTANA_BOUNDS.maxLongitude;
}

function locationLabel(position, prefix = "Location") {
  if (!position) {
    return "";
  }
  return `${prefix}: ${Number(position.latitude).toFixed(6)}, ${Number(position.longitude).toFixed(6)}`;
}

async function reverseGeocode(position) {
  if (!position || !GIS_API_KEY) {
    return locationLabel(position, "Location");
  }
  const params = new URLSearchParams({
    lat: String(position.latitude),
    lon: String(position.longitude),
    radius: "1200",
    type: "building,street,road,crossroad,gate,parking",
    fields: "items.point,items.address,items.full_address_name,items.adm_div",
    key: GIS_API_KEY,
  });
  const response = await fetch(`https://catalog.api.2gis.com/3.0/items/geocode?${params}`);
  if (!response.ok) {
    return locationLabel(position, "Location");
  }
  const data = await response.json();
  const items = data?.result?.items || [];
  const exact = items
    .map(formatGeocoderItem)
    .find((value) => value && !isBroadAddress(value));
  return exact || "No exact street or building found. Pick closer to a road or building.";
}

async function forwardGeocode(address) {
  const query = String(address || "").trim();
  if (!query || !GIS_API_KEY) {
    return null;
  }
  const params = new URLSearchParams({
    q: `${query}, Астана`,
    fields: "items.point,items.address,items.full_address_name,items.adm_div",
    key: GIS_API_KEY,
  });
  const response = await fetch(`https://catalog.api.2gis.com/3.0/items/geocode?${params}`);
  if (!response.ok) {
    return null;
  }
  const data = await response.json();
  const item = (data?.result?.items || []).find((nextItem) => {
    const point = nextItem?.point;
    return point && Number.isFinite(Number(point.lat)) && Number.isFinite(Number(point.lon));
  });
  if (!item?.point) {
    return null;
  }
  return {
    latitude: Number(item.point.lat),
    longitude: Number(item.point.lon),
    label: formatGeocoderItem(item) || query,
  };
}

async function buildDrivingRoute(start, end) {
  if (!start || !end || !GIS_API_KEY) {
    return directRoute(start, end);
  }
  const payloads = [
    {
      points: [
        { type: "stop", lon: start.longitude, lat: start.latitude },
        { type: "stop", lon: end.longitude, lat: end.latitude },
      ],
      transport: "driving",
      route_mode: "fastest",
      traffic_mode: "jam",
      locale: "ru",
    },
    {
      points: [
        { type: "walking", x: start.longitude, y: start.latitude },
        { type: "walking", x: end.longitude, y: end.latitude },
      ],
      transport: "car",
      route_mode: "fastest",
      traffic_mode: "jam",
      locale: "ru",
    },
  ];
  try {
    for (let attempt = 0; attempt < 3; attempt += 1) {
      for (const payload of payloads) {
        const response = await fetch(`https://routing.api.2gis.com/routing/7.0.0/global?key=${encodeURIComponent(GIS_API_KEY)}`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        });
        if (!response.ok) {
          continue;
        }
        const data = await response.json();
        const points = collectRoutePoints(data?.result?.[0] || data?.result || data);
        if (points.length >= 2) {
          return points;
        }
      }
      await wait(350);
    }
    return directRoute(start, end);
  } catch {
    return directRoute(start, end);
  }
}

function wait(ms) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

function collectRoutePoints(route) {
  const points = [];
  const add = (value) => {
    const nextPoints = parseLineString(value);
    if (nextPoints.length > 0) {
      points.push(...nextPoints);
    }
  };

  add(route?.begin_pedestrian_path?.geometry?.selection);
  for (const maneuver of route?.maneuvers || []) {
    add(maneuver?.outcoming_path?.geometry?.selection);
    for (const part of maneuver?.outcoming_path?.geometry || []) {
      add(part?.selection);
      add(part);
    }
    add(maneuver?.geometry?.selection);
    for (const part of maneuver?.geometry || []) {
      add(part?.selection);
      add(part);
    }
  }
  add(route?.end_pedestrian_path?.geometry?.selection);
  if (points.length === 0) {
    collectRouteGeometry(route, add);
  }

  return dedupeRoutePoints(points);
}

function collectRouteGeometry(value, add) {
  if (!value) {
    return;
  }
  if (typeof value === "string") {
    add(value);
    return;
  }
  if (Array.isArray(value)) {
    value.forEach((item) => collectRouteGeometry(item, add));
    return;
  }
  if (typeof value === "object") {
    Object.values(value).forEach((item) => collectRouteGeometry(item, add));
  }
}

function parseLineString(value) {
  const match = String(value || "").match(/LINESTRING\s*\(([^)]+)\)/i);
  if (!match) {
    return [];
  }
  return match[1]
    .split(",")
    .map((pair) => {
      const [longitude, latitude] = pair.trim().split(/\s+/).map(Number);
      if (!Number.isFinite(latitude) || !Number.isFinite(longitude)) {
        return null;
      }
      return { latitude, longitude };
    })
    .filter(Boolean);
}

function dedupeRoutePoints(points) {
  const result = [];
  for (const point of points) {
    const prev = result[result.length - 1];
    if (!prev || Math.abs(prev.latitude - point.latitude) > 0.000001 || Math.abs(prev.longitude - point.longitude) > 0.000001) {
      result.push(point);
    }
  }
  return result;
}

function directRoute(start, end) {
  if (!start || !end) {
    return null;
  }
  return [
    { latitude: start.latitude, longitude: start.longitude },
    { latitude: end.latitude, longitude: end.longitude },
  ];
}

function pickCompletionPhoto() {
  return new Promise((resolve) => {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = "image/jpeg,image/png,image/webp";
    input.onchange = () => resolve(input.files?.[0] || null);
    input.click();
  });
}

function formatGeocoderItem(item) {
  if (!item || String(item.type || "").startsWith("adm_div")) {
    return "";
  }
  if (item.full_address_name) {
    return item.full_address_name;
  }
  if (item.address_name) {
    return item.address_name;
  }
  if (item.address?.building_name) {
    return item.address.building_name;
  }
  const components = item.address?.components || [];
  const street = components.find((part) => part.type === "street")?.name;
  const number = components.find((part) => part.type === "street_number")?.name;
  if (street && number) {
    return `${street}, ${number}`;
  }
  if (street) {
    return street;
  }
  return item.name || "";
}

function isBroadAddress(value) {
  const normalized = String(value).toLowerCase();
  return normalized === "астана, есиль" ||
    normalized === "astana, esil" ||
    normalized === "астана" ||
    normalized === "astana";
}

function workerRouteLine(position, bookings) {
  const active = (bookings || []).find((booking) => String(booking.status || "").toLowerCase() === "in_progress" && booking.latitude && booking.longitude);
  if (!position || !active) {
    return null;
  }
  return [
    { latitude: position.latitude, longitude: position.longitude },
    { latitude: active.latitude, longitude: active.longitude },
  ];
}

function activeInProgressBooking(bookings) {
  return (bookings || []).find((booking) => String(booking.status || booking.booking_status || "").toLowerCase() === "in_progress");
}

function isOpenBooking(booking) {
  const status = String(booking?.status || booking?.booking_status || "").toLowerCase();
  return status === "scheduled" || status === "in_progress" || status === "awaiting_confirmation";
}

function canChatBooking(booking) {
  const status = String(booking?.status || booking?.booking_status || "").toLowerCase();
  return status === "price_pending" || status === "scheduled" || status === "in_progress" || status === "awaiting_confirmation";
}

async function ensureWorkerProfile(token, position) {
  await apiPost("/api/worker/profile", token, {
    bio: "",
    current_latitude: position?.latitude || 0,
    current_longitude: position?.longitude || 0,
  });
}

function clearAuthURL() {
  if (window.location.pathname !== "/" || window.location.search) {
    window.history.replaceState({}, "", "/");
  }
}

function readPaymentSetupIntent() {
  try {
    const raw = localStorage.getItem(PAYMENT_SETUP_INTENT_KEY);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

function defaultTabForRole(role) {
  if (role === "worker") {
    return "pro";
  }
  if (role === "admin") {
    return "overview";
  }
  if (role === "manager") {
    return "overview";
  }
  return "find";
}

function AppFrame({ role, session, activeTab, onTab, onSignOut, children }) {
  const tabs = roleTabs[role] || [];
  if (role === "worker" || role === "customer") {
    return <main className="workerFullscreen">{children}</main>;
  }
  return (
    <main className="adminShell">
      <header className="adminTopbar">
        <div className="brandBlock compactBrand">
          <div className="appIcon">WM</div>
          <div>
            <h1>{role === "admin" ? "Admin workspace" : "Manager workspace"}</h1>
            <p>{session.email || "Signed in"} - {role || "no role"}</p>
          </div>
        </div>
        <nav className="adminTabs">
          {tabs.map(([id, label]) => (
            <button key={id} className={activeTab === id ? "active" : ""} onClick={() => onTab(id)}>
              {label}
            </button>
          ))}
        </nav>
        <button className="ghostButton fitButton" onClick={onSignOut}>Sign out</button>
      </header>
      <section className="dashboardBody">{children}</section>
    </main>
  );
}

function CustomerApp({
  token,
  activeTab,
  onNavigate,
  onSignOut,
  paymentReady,
  paymentLoading,
  paymentError,
  onStartPaymentSetup,
}) {
  const { position, geoStatus, geoError, locate, startWatch } = useGeolocation();
  const mapRef = useRef(null);
  const [categories, setCategories] = useState([]);
  const pendingPaymentIntent = useMemo(() => readPaymentSetupIntent(), []);
  const [categoryID, setCategoryID] = useState(pendingPaymentIntent?.categoryID || "");
  const [locationMode, setLocationMode] = useState(pendingPaymentIntent?.locationMode || "current");
  const [pickedPosition, setPickedPosition] = useState(pendingPaymentIntent?.pickedPosition || null);
  const [workers, setWorkers] = useState([]);
  const [selectedWorker, setSelectedWorker] = useState(null);
  const [profileWorker, setProfileWorker] = useState(null);
  const [bookings, setBookings] = useState([]);
  const [description, setDescription] = useState(pendingPaymentIntent?.description || "");
  const [address, setAddress] = useState(pendingPaymentIntent?.address || "");
  const [addressDraft, setAddressDraft] = useState(pendingPaymentIntent?.address || "");
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [routePoints, setRoutePoints] = useState(null);

  useEffect(() => {
    apiGet("/api/categories", token).then(setCategories).catch((err) => setError(err.message));
  }, [token]);

  const loadCustomerBookings = useCallback(() => {
    apiGet("/api/bookings/my", token)
      .then((data) => setBookings(Array.isArray(data) ? data : data.bookings || []))
      .catch(() => {});
  }, [token]);

  useEffect(() => {
    loadCustomerBookings();
    const intervalID = window.setInterval(loadCustomerBookings, 2000);
    return () => window.clearInterval(intervalID);
  }, [loadCustomerBookings]);

  useEffect(() => {
    let cancelled = false;
    if (position && locationMode === "current" && !address) {
      reverseGeocode(position).then((nextAddress) => {
        if (!cancelled) {
          setAddress(nextAddress);
          setAddressDraft(nextAddress);
        }
      });
    }
    return () => {
      cancelled = true;
    };
  }, [address, locationMode, position]);

  useEffect(() => {
    const query = addressDraft.trim();
    if (activeTab !== "find" || locationMode !== "address" || query.length < 3) {
      return undefined;
    }
    let cancelled = false;
    const timerID = window.setTimeout(async () => {
      const result = await forwardGeocode(query);
      if (cancelled || !result) {
        return;
      }
      const nextPosition = { latitude: result.latitude, longitude: result.longitude };
      setPickedPosition(nextPosition);
      setAddress(result.label);
    }, 650);
    return () => {
      cancelled = true;
      window.clearTimeout(timerID);
    };
  }, [activeTab, addressDraft, locationMode]);

  const activeCustomerBooking = bookings.find((item) => {
    const status = String(item.status || "").toLowerCase();
    return status === "in_progress" && item.worker_latitude && item.worker_longitude && item.latitude && item.longitude;
  }) || bookings.find((item) => {
    const status = String(item.status || "").toLowerCase();
    return (status === "scheduled" || status === "awaiting_confirmation") && item.worker_latitude && item.worker_longitude;
  });
  const trackingWorker = activeCustomerBooking ? [{
    worker_id: activeCustomerBooking.worker_profile_id,
    full_name: activeCustomerBooking.counterparty_name || "Worker",
    latitude: activeCustomerBooking.worker_latitude,
    longitude: activeCustomerBooking.worker_longitude,
    distance_meters: 0,
  }] : null;
  const customerDestination = activeCustomerBooking?.latitude && activeCustomerBooking?.longitude
    ? { latitude: activeCustomerBooking.latitude, longitude: activeCustomerBooking.longitude }
    : null;
  const searchPosition = locationMode === "map" || locationMode === "address" ? pickedPosition : position;

  useEffect(() => {
    let cancelled = false;
    const bookingStatus = String(activeCustomerBooking?.status || "").toLowerCase();
    if (!activeCustomerBooking || !customerDestination || bookingStatus === "scheduled") {
      setRoutePoints(null);
      return () => {
        cancelled = true;
      };
    }
    const workerPosition = {
      latitude: activeCustomerBooking.worker_latitude,
      longitude: activeCustomerBooking.worker_longitude,
    };
    buildDrivingRoute(workerPosition, customerDestination).then((points) => {
      if (!cancelled) {
        setRoutePoints(points);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [
    activeCustomerBooking?.booking_id,
    activeCustomerBooking?.status,
    activeCustomerBooking?.worker_latitude,
    activeCustomerBooking?.worker_longitude,
    customerDestination?.latitude,
    customerDestination?.longitude,
  ]);

  const useCurrentLocation = async () => {
    const nextPosition = await locate();
    setLocationMode("current");
    setPickedPosition(null);
    if (nextPosition) {
      const nextAddress = await reverseGeocode(nextPosition);
      setAddress(nextAddress);
      setAddressDraft(nextAddress);
      mapRef.current?.recenter();
    }
  };

  const pickMapPosition = async (nextPosition) => {
    setPickedPosition(nextPosition);
    setLocationMode("map");
    setAddress("Resolving address...");
    const nextAddress = await reverseGeocode(nextPosition);
    setAddress(nextAddress);
    setAddressDraft(nextAddress);
  };

  const searchWorkers = async () => {
    if (paymentLoading) {
      setError("Checking payment method. Try again in a moment.");
      return;
    }
    if (!paymentReady) {
      setError(paymentError || "Link a payment card before searching workers.");
      localStorage.setItem(PAYMENT_SETUP_INTENT_KEY, JSON.stringify({
        action: "search",
        categoryID,
        locationMode,
        pickedPosition,
        address,
        description,
      }));
      try {
        await onStartPaymentSetup?.();
      } catch (err) {
        setError(err.message);
      }
      return;
    }
    if (!searchPosition || !categoryID) {
      setError("Choose category and location first.");
      return;
    }
    if (!isInsideAstana(searchPosition)) {
      setError("Service is available only in Astana.");
      return;
    }
    setLoading(true);
    setError("");
    setMessage("");
    try {
      const query = new URLSearchParams({
        category_id: categoryID,
        latitude: String(searchPosition.latitude),
        longitude: String(searchPosition.longitude),
      });
      const data = await apiGet(`/api/geo/workers/nearby?${query}`, token);
      setWorkers(Array.isArray(data) ? data : data.workers || []);
      localStorage.removeItem(PAYMENT_SETUP_INTENT_KEY);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const hireWorker = async (worker) => {
    if (!searchPosition || !categoryID) {
      setError("Location and category are required.");
      return;
    }
    if (!isInsideAstana(searchPosition)) {
      setError("Orders are accepted only in Astana.");
      return;
    }
    setError("");
    try {
      const request = await apiPost("/api/requests", token, {
        category_id: Number(categoryID),
        description: description || `Request for ${worker.full_name}`,
        address: address || "Customer location",
        latitude: searchPosition.latitude,
        longitude: searchPosition.longitude,
      });
      const booking = await apiPost("/api/bookings", token, {
        request_id: Number(request.request_id || request.id),
        worker_profile_id: Number(worker.worker_id || worker.worker_profile_id),
      });
      const bookingID = booking.booking_id || booking.id;
      if (bookingID) {
        const chat = await apiPost("/api/chats", token, { booking_id: Number(bookingID) });
        if (chat?.chat_id) {
          localStorage.setItem("workers_marketplace_active_chat", String(chat.chat_id));
        }
        onNavigate("chats");
      }
      setMessage(`Chat opened with ${worker.full_name}. Agree on the price before booking starts.`);
      loadCustomerBookings();
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => {
    const pending = readPaymentSetupIntent();
    if (!paymentReady || pending?.action !== "search" || activeTab !== "find") {
      return;
    }
    const timerID = window.setTimeout(() => {
      searchWorkers();
    }, 300);
    return () => window.clearTimeout(timerID);
  }, [activeTab, paymentReady]);

  if (activeTab === "requests") return <CustomerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><RequestsPanel token={token} /></CustomerPhonePage>;
  if (activeTab === "bookings") return <CustomerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><BookingsPanel token={token} canConfirm onNavigate={onNavigate} /></CustomerPhonePage>;
  if (activeTab === "chats") return <CustomerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><ChatPanel token={token} role="customer" /></CustomerPhonePage>;
  if (activeTab === "worker-profile") return <CustomerPhonePage activeTab="find" onNavigate={onNavigate} onSignOut={onSignOut}><WorkerPublicProfilePage token={token} worker={profileWorker} onBack={() => onNavigate("find")} onHireWorker={hireWorker} /></CustomerPhonePage>;
  if (activeTab === "profile") return <CustomerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><CustomerProfilePanel token={token} onNavigate={onNavigate} /></CustomerPhonePage>;
  if (activeTab === "notifications") return <CustomerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><NotificationsPanel token={token} onNavigate={onNavigate} /></CustomerPhonePage>;

  if (!position) {
    return <CustomerLocationGate geoStatus={geoStatus} geoError={geoError} onAllow={startWatch} onSignOut={onSignOut} />;
  }

  return (
    <div className="proPhoneShell">
      <section className="proPhone customerProPhone" aria-label="Customer map workspace">
        <MapView
          ref={mapRef}
          position={customerDestination || searchPosition || position}
          workers={trackingWorker || workers}
          selectedWorker={trackingWorker?.[0] || selectedWorker}
          onSelectWorker={setSelectedWorker}
          userMarker={locationMode === "map" || locationMode === "address" ? "none" : "default"}
          pickMode={locationMode === "map"}
          pickedPosition={pickedPosition}
          onPickPosition={pickMapPosition}
          autoCenterOnPosition={false}
          routeLine={routePoints}
        />
        <CustomerPhoneTabs activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut} />
        <button className="roundMapButton plusButton" onClick={() => mapRef.current?.zoomIn()}>+</button>
        <button className="roundMapButton minusButton" onClick={() => mapRef.current?.zoomOut()}>-</button>
        <button className="roundMapButton navButtonMap" onClick={useCurrentLocation}>GPS</button>
        <div className="offersDrawer customerOffersDrawer">
          <div className="dockHeader">
            <div>
              <h2>Find a worker</h2>
              <p>Choose service, search nearby and book.</p>
            </div>
            <button className="walletButton" onClick={searchWorkers} disabled={loading}>Search</button>
          </div>
          <div className="locationModeTabs">
            <button className={locationMode === "current" ? "active" : ""} onClick={useCurrentLocation}>Current location</button>
            <button className={locationMode === "map" ? "active" : ""} onClick={() => setLocationMode("map")}>Pick on map</button>
          </div>
          <div className="customerSearchGrid">
            <Field label="Category" light>
              <select value={categoryID} onChange={(e) => setCategoryID(e.target.value)}>
                <option value="">Choose category</option>
                {categories.map((category) => (
                  <option key={category.category_id} value={category.category_id}>{categoryTitle(category.name)}</option>
                ))}
              </select>
            </Field>
            <Field label="Address" light><input value={addressDraft} onChange={(e) => {
              setAddressDraft(e.target.value);
              setAddress(e.target.value);
              setLocationMode("address");
            }} placeholder="Arrival address" /></Field>
            <Field label="Task" light><input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Describe task" /></Field>
          </div>
          {locationMode === "map" && <p className="muted">Click on the map to choose the arrival point.</p>}
          <StatusLine geoStatus={geoStatus} geoError={geoError} />
          <Messages message={message} error={error} />
          <WorkerList
            workers={workers}
            selectedWorker={selectedWorker}
            onSelectWorker={setSelectedWorker}
            onHireWorker={hireWorker}
            onOpenProfile={(worker) => {
              setProfileWorker(worker);
              onNavigate("worker-profile");
            }}
            loading={loading}
          />
        </div>
      </section>
    </div>
  );
}

function CustomerPhonePage({ activeTab, onNavigate, onSignOut, children }) {
  return (
    <div className="customerPhoneShell">
      <section className="customerPageScreen">
        <CustomerPhoneTabs activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut} />
        <div className="customerInnerPage">{children}</div>
      </section>
    </div>
  );
}

function WorkerPublicProfilePage({ token, worker, onBack, onHireWorker }) {
  const workerID = worker?.worker_id || worker?.worker_profile_id;
  const [profile, setProfile] = useState(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!workerID) {
      setError("Worker profile is missing.");
      return;
    }
    setError("");
    apiGet(`/api/reviews/workers/${workerID}`, token)
      .then(setProfile)
      .catch((err) => setError(err.message));
  }, [token, workerID]);

  const reviews = profile?.reviews || [];
  const skills = profile?.skills || [];
  const name = profile?.worker_name || worker?.full_name || "Worker";
  const photoURL = profile?.profile_photo_url ? apiURL(profile.profile_photo_url) : "";
  const rating = Number(profile?.average_rating || worker?.average_rating || 0);
  const reviewCount = Number(profile?.review_count || worker?.review_count || 0);

  return (
    <section className="pagePanel workerPublicProfile">
      <div className="sectionTitleRow">
        <button className="secondaryButton fitButton" type="button" onClick={onBack}>Back</button>
        <button type="button" className="fitButton" onClick={() => worker && onHireWorker(worker)}>Open chat</button>
      </div>
      <div className="publicProfileHero">
        <div className="profilePhoto compactPhoto">
          <span>WM</span>
          {photoURL ? <img src={photoURL} alt="" onError={(event) => event.currentTarget.remove()} /> : null}
        </div>
        <div>
          <h2>{name}</h2>
          <div className="ratingLine">
            <span>{renderStars(rating)}</span>
            <small>{rating.toFixed(1)} from {reviewCount} review{reviewCount === 1 ? "" : "s"}</small>
          </div>
          <p>{profile?.bio || "Worker has not added profile information yet."}</p>
          {worker?.distance_meters !== undefined && <span className="pillTag">{formatDistanceLabel(worker.distance_meters)}</span>}
        </div>
      </div>
      <section className="profileSection">
        <div className="sectionTitleRow">
          <h3>Skills</h3>
        </div>
        <div className="verifiedSkillsGrid">
          {skills.length === 0 && worker && (
            <article className="verifiedSkillCard">
              <strong>{categoryTitle(worker.category_name)}</strong>
              <span>{worker.experience_level}</span>
            </article>
          )}
          {skills.map((skill) => (
            <article className="verifiedSkillCard" key={skill.worker_skill_id}>
              <strong>{categoryTitle(skill.category_name)}</strong>
              <span>{skill.experience_level}</span>
            </article>
          ))}
        </div>
      </section>
      <section className="profileSection">
        <div className="sectionTitleRow">
          <h3>Reviews</h3>
          <span>{reviewCount}</span>
        </div>
        <div className="publicReviewList">
          {reviews.length === 0 && <EmptyState title="No reviews yet" text="Customer reviews will appear here after completed bookings." />}
          {reviews.map((review) => (
            <article className="publicReviewCard" key={review.review_id}>
              <strong>{renderStars(review.rating)} {review.customer_name || "Customer"}</strong>
              <span>{categoryTitle(review.category_name)} - {formatDateTime(review.created_at)}</span>
              {review.comment && <p>{review.comment}</p>}
              {review.photo_url && <img src={apiURL(review.photo_url)} alt="" />}
            </article>
          ))}
        </div>
      </section>
      <Messages error={error} />
    </section>
  );
}

function CustomerLocationGate({ geoStatus, geoError, onAllow, onSignOut }) {
  return (
    <main className="geoGate">
      <section className="geoGateCard">
        <div className="appIcon">WM</div>
        <h1>Allow location</h1>
        <p>We use your location to find nearby workers in Astana.</p>
        <button onClick={onAllow} disabled={geoStatus === "loading"}>{geoStatus === "loading" ? "Requesting..." : "Allow location"}</button>
        <button className="secondaryButton" onClick={onSignOut}>Exit</button>
        {geoError && <p className="errorMessage">{geoError}</p>}
      </section>
    </main>
  );
}

function CustomerPhoneTabs({ activeTab, onNavigate, onSignOut }) {
  return (
    <div className="workerPhoneTabs customerPhoneTabs">
      {roleTabs.customer.map(([id, label]) => (
        <button key={id} className={activeTab === id ? "active" : ""} onClick={() => onNavigate(id)}>
          {label}
        </button>
      ))}
      <button onClick={onSignOut}>Exit</button>
    </div>
  );
}

function WorkerApp({
  token,
  activeTab,
  onNavigate,
  onSignOut,
  paymentReady,
  paymentLoading,
  paymentError,
  onStartPaymentSetup,
}) {
  const { position, geoStatus, geoError, startWatch } = useGeolocation();
  const mapRef = useRef(null);
  const [available, setAvailable] = useState(false);
  const [searching, setSearching] = useState(false);
  const [bookings, setBookings] = useState([]);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [routePoints, setRoutePoints] = useState(null);
  const knownScheduledBookingsRef = useRef(null);

  const loadBookings = useCallback(async () => {
    setError("");
    try {
      const data = await apiGet("/api/bookings/my", token);
      const nextBookings = Array.isArray(data) ? data : data.bookings || [];
      announceNewScheduledBookings(nextBookings, knownScheduledBookingsRef);
      setBookings(nextBookings);
      if (activeInProgressBooking(nextBookings)) {
        setAvailable(false);
        setSearching(false);
      } else if (nextBookings.length > 0) {
        setSearching(false);
      }
    } catch (err) {
      if (!isEmptyResultError(err)) {
        setError(err.message);
      }
    }
  }, [token]);

  useEffect(() => {
    loadBookings();
  }, [loadBookings]);

  useEffect(() => {
    const intervalID = window.setInterval(loadBookings, 2000);
    return () => window.clearInterval(intervalID);
  }, [loadBookings]);

  const syncLocation = useCallback(async () => {
    if (!position) {
      setError("Location is not ready.");
      return;
    }
    setError("");
    try {
      await apiPatch("/api/geo/worker/location", token, {
        latitude: position.latitude,
        longitude: position.longitude,
      });
      setMessage("Location updated.");
    } catch (err) {
      setError(err.message);
    }
  }, [position, token]);

  const currentInProgressBooking = activeInProgressBooking(bookings);

  useEffect(() => {
    let cancelled = false;
    if (!position || !currentInProgressBooking?.latitude || !currentInProgressBooking?.longitude) {
      setRoutePoints(null);
      return () => {
        cancelled = true;
      };
    }
    const destination = {
      latitude: currentInProgressBooking.latitude,
      longitude: currentInProgressBooking.longitude,
    };
    buildDrivingRoute(position, destination).then((points) => {
      if (!cancelled) {
        setRoutePoints((current) => {
          if (isDirectRoute(points) && Array.isArray(current) && current.length > 2) {
            return current;
          }
          return points;
        });
      }
    });
    return () => {
      cancelled = true;
    };
  }, [
    position?.latitude,
    position?.longitude,
    currentInProgressBooking?.booking_id,
    currentInProgressBooking?.latitude,
    currentInProgressBooking?.longitude,
  ]);

  useEffect(() => {
    if (!position || !currentInProgressBooking?.latitude || !currentInProgressBooking?.longitude) {
      return;
    }
    announceNavigationHint(position, {
      latitude: currentInProgressBooking.latitude,
      longitude: currentInProgressBooking.longitude,
    });
  }, [
    position?.latitude,
    position?.longitude,
    currentInProgressBooking?.booking_id,
    currentInProgressBooking?.latitude,
    currentInProgressBooking?.longitude,
  ]);

  useEffect(() => {
    if ((!available && !currentInProgressBooking) || !position) {
      return undefined;
    }
    syncLocation();
    const intervalID = window.setInterval(() => {
      syncLocation();
      if (searching) {
        loadBookings();
      }
    }, 4000);
    return () => window.clearInterval(intervalID);
  }, [available, currentInProgressBooking, loadBookings, position, searching, syncLocation]);

  const toggleAvailability = async () => {
    setError("");
    try {
      const next = !available;
      if (next) {
        if (paymentLoading) {
          setError("Checking payment method. Try again in a moment.");
          return;
        }
        if (!paymentReady) {
          setError(paymentError || "Link a payment card before going online.");
          await onStartPaymentSetup?.();
          return;
        }
        let nextBookings = [];
        try {
          const data = await apiGet("/api/bookings/my", token);
          nextBookings = Array.isArray(data) ? data : data.bookings || [];
        } catch (err) {
          if (!isEmptyResultError(err)) {
            throw err;
          }
        }
        setBookings(nextBookings);
        if (activeInProgressBooking(nextBookings)) {
          setAvailable(false);
          setSearching(false);
          setMessage("Offline.");
          setError("You already have a job in progress. Finish it before going online.");
          return;
        }
      }
      try {
        await apiPatch("/api/worker/availability", token, { is_available: next });
      } catch (err) {
        if (!next || !isMissingWorkerProfileError(err)) {
          throw err;
        }
        await ensureWorkerProfile(token, position);
        await apiPatch("/api/worker/availability", token, { is_available: next });
      }
      setAvailable(next);
      setSearching(next);
      setMessage(next ? "Online. Searching jobs..." : "Offline.");
      if (next) {
        await syncLocation();
        await loadBookings();
      }
    } catch (err) {
      setError(err.message);
    }
  };

  const updateBooking = async (bookingID, action) => {
    setError("");
    try {
      if (action === "complete") {
        const file = await pickCompletionPhoto();
        if (!file) {
          setError("Completion photo is required.");
          return;
        }
        const body = new FormData();
        body.append("evidence_file", file);
        await apiMultipartPatch(`/api/bookings/${bookingID}/complete`, token, body);
        setMessage("Proof photo sent. Waiting for customer confirmation.");
      } else {
        await apiPatch(`/api/bookings/${bookingID}/${action}`, token, {});
        setMessage(action === "reject" ? "Booking rejected." : `Booking ${action}ed.`);
        if (action === "start") {
          setSearching(false);
          setAvailable(false);
          onNavigate("pro");
        }
      }
      loadBookings();
    } catch (err) {
      setError(err.message);
    }
  };

  if (activeTab === "jobs") {
    return <WorkerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><BookingsPanel token={token} canProgress onProgress={updateBooking} onNavigate={onNavigate} /></WorkerPhonePage>;
  }
  if (activeTab === "chats") {
    return <WorkerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><ChatPanel token={token} role="worker" /></WorkerPhonePage>;
  }
  if (activeTab === "skills") {
    return <WorkerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><WorkerSkillsPanel token={token} /></WorkerPhonePage>;
  }
  if (activeTab === "profile") {
    return <WorkerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><WorkerProfilePanel token={token} onNavigate={onNavigate} /></WorkerPhonePage>;
  }
  if (activeTab === "notifications") {
    return <WorkerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><NotificationsPanel token={token} onNavigate={onNavigate} /></WorkerPhonePage>;
  }

  if (!position) {
    return <WorkerLocationGate geoStatus={geoStatus} geoError={geoError} onAllow={startWatch} onSignOut={onSignOut} />;
  }

  return (
    <div className="proPhoneShell">
      <section className="proPhone" aria-label="Worker Pro map workspace">
        <MapView
          ref={mapRef}
          position={position}
          workers={[]}
          selectedWorker={null}
          onSelectWorker={() => {}}
          userMarker="driver"
          routeLine={routePoints}
          routeFocusKey={currentInProgressBooking?.booking_id || ""}
          autoCenterOnPosition={!currentInProgressBooking}
        />
        <WorkerPhoneTabs activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut} />
        {available && searching && bookings.length === 0 && (
          <div className="searchPulse" aria-hidden="true">
            <span />
            <span />
            <span />
          </div>
        )}
        <button className={available ? "searchButton lineSearchButton online" : "searchButton lineSearchButton"} onClick={toggleAvailability}>
          {available ? "Offline" : "Go online"}
        </button>
        <button className="roundMapButton plusButton" onClick={() => mapRef.current?.zoomIn()}>+</button>
        <button className="roundMapButton minusButton" onClick={() => mapRef.current?.zoomOut()}>-</button>
        <button className="roundMapButton navButtonMap" onClick={() => mapRef.current?.recenter()}>GPS</button>
        <div className="offersDrawer">
          <div>
            <h2>Offers</h2>
            <button className="walletButton" onClick={() => onNavigate("jobs")}>Jobs</button>
          </div>
          <Messages message={message} error={error} />
          <JobBoard bookings={bookings.filter(isOpenBooking).slice(0, 2)} onProgress={updateBooking} compact />
        </div>
      </section>
    </div>
  );
}

function WorkerPhonePage({ activeTab, onNavigate, onSignOut, children }) {
  return (
    <div className="proPhoneShell">
      <section className="proPhone workerPagePhone">
        <WorkerPhoneTabs activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut} />
        <div className="workerInnerPage">{children}</div>
      </section>
    </div>
  );
}

function WorkerLocationGate({ geoStatus, geoError, onAllow, onSignOut }) {
  const loading = geoStatus === "loading";
  return (
    <main className="geoGate">
      <section className="geoGateCard">
        <div className="appIcon">WM</div>
        <h1>Allow location</h1>
        <p>We need your location for the worker map and online job search.</p>
        {loading && <div className="geoLoader" aria-hidden="true"><span /><span /></div>}
        <button onClick={onAllow} disabled={loading}>{loading ? "Finding location..." : "Allow location"}</button>
        <button className="secondaryButton" onClick={onSignOut}>Exit</button>
        {geoError && <p className="errorMessage">{geoError}</p>}
      </section>
    </main>
  );
}

function WorkerPhoneTabs({ activeTab, onNavigate, onSignOut }) {
  return (
    <div className="workerPhoneTabs">
      {roleTabs.worker.map(([id, label]) => (
        <button key={id} className={activeTab === id ? "active" : ""} onClick={() => onNavigate(id)}>
          {label}
        </button>
      ))}
      <button onClick={onSignOut}>Exit</button>
    </div>
  );
}

function AdminApp({ token, role, activeTab, onNavigate }) {
  const [overview, setOverview] = useState(null);
  const [users, setUsers] = useState([]);
  const [staffForm, setStaffForm] = useState({ full_name: "", email: "", phone: "", password: "", role: "manager" });
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const loadOverview = useCallback(() => {
    apiGet("/api/admin/overview", token).then(setOverview).catch((err) => setError(err.message));
  }, [token]);
  const loadUsers = useCallback(() => {
    apiGet("/api/admin/users", token).then(setUsers).catch((err) => setError(err.message));
  }, [token]);

  useEffect(() => {
    if (activeTab === "overview" || activeTab === "verify") {
      loadOverview();
    }
    if (activeTab === "users" || activeTab === "accounts" || activeTab === "overview") {
      loadUsers();
    }
  }, [activeTab, loadOverview, loadUsers]);

  if (activeTab === "notifications") return <NotificationsPanel token={token} onNavigate={onNavigate} />;

  const verifySkill = async (id) => {
    setError("");
    setMessage("");
    try {
      await apiPost("/api/admin/verify-skill", token, { worker_skill_id: Number(id) });
      setMessage("Skill verified. Worker profile was verified automatically.");
      loadOverview();
    } catch (err) {
      setError(err.message);
    }
  };

  const deleteUser = async (id) => {
    setError("");
    setMessage("");
    try {
      await apiDelete(`/api/admin/users/${id}`, token);
      setMessage("User deleted.");
      loadUsers();
      loadOverview();
    } catch (err) {
      setError(err.message);
    }
  };

  const activateUser = async (id) => {
    setError("");
    setMessage("");
    try {
      await apiPatch(`/api/admin/users/${id}/activate`, token, {});
      setMessage("User activated.");
      loadUsers();
      loadOverview();
    } catch (err) {
      setError(err.message);
    }
  };

  const createStaff = async (event) => {
    event.preventDefault();
    setError("");
    setMessage("");
    try {
      const endpoint = staffForm.role === "admin" ? "/api/admin/admins" : "/api/admin/managers";
      await apiPost(endpoint, token, staffForm);
      setMessage(`${staffForm.role === "admin" ? "Admin" : "Manager"} account created.`);
      setStaffForm({ full_name: "", email: "", phone: "", password: "", role: "manager" });
      loadUsers();
      loadOverview();
      onNavigate("users");
    } catch (err) {
      setError(err.message);
    }
  };

  const admins = users.filter((user) => user.role === "admin");
  const managers = users.filter((user) => user.role === "manager");
  const isAdmin = role === "admin";

  return (
    <section className="adminWorkspace">
      <SectionHeader title={activeTab === "verify" ? "Verification queue" : activeTab === "users" ? "Users" : activeTab === "accounts" ? "Staff accounts" : "Operations dashboard"} text="Support customer-worker operations, verification and account issues." />
      {activeTab === "overview" && (
        <AdminOverviewPanel overview={overview} users={users} onNavigate={onNavigate} isAdmin={isAdmin} />
      )}
      {activeTab === "verify" && (
        <AdminVerificationPanel
          overview={overview}
          verifySkill={verifySkill}
        />
      )}
      {activeTab === "users" && <AdminUsersPanel users={users} onActivate={activateUser} onDelete={deleteUser} canDelete={isAdmin} />}
      {activeTab === "accounts" && isAdmin && (
        <AdminCreatePanel
          admins={admins}
          managers={managers}
          form={staffForm}
          setForm={setStaffForm}
          onSubmit={createStaff}
          onDelete={deleteUser}
        />
      )}
      {activeTab === "accounts" && !isAdmin && <EmptyState title="Admins only" text="Managers can review queues and help users, but cannot create staff accounts." />}
      <Messages message={message} error={error} />
    </section>
  );
}

function AdminOverviewPanel({ overview, users, onNavigate, isAdmin }) {
  const stats = overview?.stats || {};
  const pendingSkills = overview?.pending_skills || [];
  return (
    <div className="adminOverview">
      <div className="profileStatsGrid">
        <StatCard title="Users" value={stats.users_total || users.length || 0} text={`${stats.customers_total || 0} customers, ${stats.workers_total || 0} workers`} />
        <StatCard title="Verification" value={stats.pending_worker_skills || 0} text="Pending skill evidence" />
        <StatCard title="Bookings" value={stats.bookings_total || 0} text={`${stats.bookings_in_progress || 0} in progress`} />
      </div>
      <div className="adminActionGrid">
        <button className="adminActionCard" onClick={() => onNavigate("verify")}>
          <strong>Review queue</strong>
          <span>{pendingSkills.length} skills waiting</span>
        </button>
        <button className="adminActionCard" onClick={() => onNavigate("users")}>
          <strong>User support</strong>
          <span>Find customers, workers, managers and admins</span>
        </button>
        {isAdmin && <button className="adminActionCard" onClick={() => onNavigate("accounts")}>
          <strong>Manager access</strong>
          <span>Create manager and admin accounts for the team</span>
        </button>}
      </div>
      <div className="adminTwoColumn">
        <AdminMiniQueue title="Pending skills" items={pendingSkills} empty="No skill evidence to review." skill />
      </div>
    </div>
  );
}

function AdminMiniQueue({ title, items, empty, skill }) {
  return (
    <section className="toolCard">
      <h3>{title}</h3>
      <div className="dataList">
        {items.length === 0 && <EmptyState title={empty} text="Everything is calm here." />}
        {items.slice(0, 5).map((item) => (
          <article className="dataRow" key={skill ? item.worker_skill_id : item.worker_profile_id}>
            <strong>{skill ? categoryTitle(item.category_name) : item.full_name}</strong>
            <span>{skill ? item.worker_user_email : item.email}</span>
            <span>{skill ? `Skill #${item.worker_skill_id}` : `Worker #${item.worker_profile_id}`}</span>
          </article>
        ))}
      </div>
    </section>
  );
}

function AdminUsersPanel({ users, onActivate, onDelete, canDelete }) {
  return (
    <div className="adminUsersPanel">
      <div className="dataList adminUserList">
        {users.length === 0 && <EmptyState title="No users found" text="Registered accounts will appear here." />}
        {users.map((user) => (
          <article className="adminUserRow" key={user.user_id}>
            <div>
              <strong>{user.full_name}</strong>
              <span>{user.email}</span>
            </div>
            <span className={`rolePill ${user.role}`}>{user.role}</span>
            <span>{user.status}</span>
            <div className="adminUserActions">
              {user.status !== "active" && <button onClick={() => onActivate(user.user_id)}>Activate</button>}
              {canDelete && <button className="dangerButton" onClick={() => onDelete(user.user_id)}>Delete</button>}
            </div>
          </article>
        ))}
      </div>
    </div>
  );
}

function AdminCreatePanel({ admins, managers, form, setForm, onSubmit, onDelete }) {
  return (
    <div className="adminTwoColumn">
      <form className="toolCard adminCreateForm" onSubmit={onSubmit}>
        <h3>Create staff account</h3>
        <p className="muted">Managers can review support queues. Admins can additionally delete users and create staff accounts.</p>
        <Field label="Account type" light>
          <select value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value })}>
            <option value="manager">Manager</option>
            <option value="admin">Admin</option>
          </select>
        </Field>
        <Field label="Full name" light><input value={form.full_name} onChange={(e) => setForm({ ...form, full_name: e.target.value })} required /></Field>
        <Field label="Email" light><input type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required /></Field>
        <Field label="Phone" light><input value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} /></Field>
        <Field label="Temporary password" light><input type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required /></Field>
        <button>Create account</button>
      </form>
      <section className="toolCard">
        <h3>Current staff</h3>
        <div className="dataList">
          {managers.map((manager) => (
            <article className="dataRow" key={manager.user_id}>
              <strong>{manager.full_name}</strong>
              <span>{manager.email}</span>
              <span className="rolePill manager">manager</span>
              <button className="dangerButton" onClick={() => onDelete(manager.user_id)}>Delete manager</button>
            </article>
          ))}
          {admins.map((admin) => (
            <article className="dataRow" key={admin.user_id}>
              <strong>{admin.full_name}</strong>
              <span>{admin.email}</span>
              <span className="rolePill admin">admin</span>
              <button className="dangerButton" onClick={() => onDelete(admin.user_id)}>Delete admin</button>
            </article>
          ))}
        </div>
      </section>
    </div>
  );
}

function RequestsPanel({ token }) {
  const [items, setItems] = useState([]);
  const [error, setError] = useState("");

  useEffect(() => {
    apiGet("/api/requests/my", token)
      .then((data) => setItems(Array.isArray(data) ? data : data.requests || []))
      .catch((err) => setError(err.message));
  }, [token]);

  return (
    <section className="pagePanel">
      <SectionHeader title="My requests" text="Track created service requests." />
      <Messages error={error} />
      <div className="requestGrid">
        {items.length === 0 && <EmptyState title="No service requests yet" text="Requests appear here after you choose a worker." />}
        {items.map((item) => (
          <article className="requestCard" key={item.request_id}>
            <div className="requestCardTop">
              <strong>{categoryTitle(item.category_name) || `Request #${item.request_id}`}</strong>
              <span className={`statusPill ${String(item.status || "").toLowerCase()}`}>{item.status || "pending"}</span>
            </div>
            <p>{item.description || "No task description"}</p>
            {item.address && <span>{item.address}</span>}
            <small>{formatDateTime(item.created_at)}</small>
          </article>
        ))}
      </div>
    </section>
  );
}

function AdminVerificationPanel({
  overview,
  verifySkill,
}) {
  const pendingSkills = overview?.pending_skills || [];

  return (
    <div className="adminVerifyGrid">
      <div className="toolCard">
        <h3>Pending skills</h3>
        <p className="muted">Approve skills after checking evidence. Worker profile is verified automatically with the approved skill.</p>
        <div className="dataList">
          {pendingSkills.length === 0 && <EmptyState title="No pending skills" text="New worker skills will appear here." />}
          {pendingSkills.map((skill) => (
            <article className="dataRow" key={skill.worker_skill_id}>
              <strong>{categoryTitle(skill.category_name)}</strong>
              <span>{skill.worker_full_name} - {skill.worker_user_email}</span>
              <span>{skill.experience_level} - {skill.price_base} KZT - Skill #{skill.worker_skill_id}</span>
              <EvidenceLinks value={skill.evidence_files} />
              <button onClick={() => verifySkill(skill.worker_skill_id)}>Verify skill and worker</button>
            </article>
          ))}
        </div>
      </div>
    </div>
  );
}

function EvidenceLinks({ value }) {
  const files = String(value || "").split(",").filter(Boolean);
  if (files.length === 0) {
    return <span>No evidence files</span>;
  }
  return (
    <div className="evidenceLinks">
      {files.map((file) => (
        <a key={file} href={apiURL(file)} target="_blank" rel="noreferrer">Open evidence</a>
      ))}
    </div>
  );
}

function BookingsPanel({ token, canProgress, canConfirm, onProgress, onNavigate }) {
  const [items, setItems] = useState([]);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  const load = useCallback(() => {
    apiGet("/api/bookings/my", token).then((data) => setItems(Array.isArray(data) ? data : data.bookings || [])).catch((err) => setError(err.message));
  }, [token]);

  useEffect(() => load(), [load]);

  const confirmCompletion = async (id) => {
    setError("");
    setMessage("");
    try {
      const result = await apiPatch(`/api/bookings/${id}/confirm`, token, {});
      if (result?.payment_url) {
        window.location.href = result.payment_url;
        return;
      }
      setMessage("Booking completed. Payment was created.");
      load();
    } catch (err) {
      setError(err.message);
    }
  };

  const openChat = async (bookingID) => {
    setError("");
    setMessage("");
    try {
      const chat = await apiPost("/api/chats", token, { booking_id: Number(bookingID) });
      if (chat?.chat_id) {
        localStorage.setItem("workers_marketplace_active_chat", String(chat.chat_id));
      }
      setMessage("Chat opened.");
      onNavigate?.("chats");
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="pagePanel">
      <SectionHeader title="Bookings" text="Current and past bookings." />
      <Messages message={message} error={error} />
      <div className="dataList">
        {items.length === 0 && <EmptyState title="No bookings" text="Bookings will appear here after customer selects a worker." />}
        {items.map((item) => (
          <article className="dataRow" key={item.booking_id || item.id}>
            <strong>Booking #{item.booking_id || item.id}</strong>
            <span>Status: {item.status || item.booking_status || "unknown"}</span>
            <span>Price: {item.final_price ? `${formatMoney(parseMoney(item.final_price))} KZT` : "set in chat"}</span>
            <small>{item.description || item.address || ""}</small>
            {item.completion_evidence && <EvidenceLinks value={item.completion_evidence} />}
            {canProgress && (
              <div className="rowActions">
                <button className="secondaryButton" onClick={() => openChat(item.booking_id || item.id)}>Chat</button>
                {String(item.status).toLowerCase() === "scheduled" && <button onClick={() => onProgress(item.booking_id || item.id, "start")}>Start route</button>}
                {String(item.status).toLowerCase() === "in_progress" && <button className="secondaryButton" onClick={() => onProgress(item.booking_id || item.id, "complete")}>Send proof</button>}
              </div>
            )}
            {canConfirm && canChatBooking(item) && (
              <div className="rowActions">
                <button className="secondaryButton" onClick={() => openChat(item.booking_id || item.id)}>Chat</button>
              </div>
            )}
            {canConfirm && String(item.status).toLowerCase() === "awaiting_confirmation" && (
              <div className="rowActions">
                <button onClick={() => confirmCompletion(item.booking_id || item.id)}>Confirm completion</button>
              </div>
            )}
            {canConfirm && String(item.status).toLowerCase() === "completed" && (
              <BookingReviewBlock item={item} token={token} onSaved={load} />
            )}
          </article>
        ))}
      </div>
    </section>
  );
}

function BookingReviewBlock({ item, token, onSaved }) {
  const [rating, setRating] = useState(item.review_rating || 5);
  const [comment, setComment] = useState(item.review_comment || "");
  const [reviewPhoto, setReviewPhoto] = useState(null);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  const saveReview = async () => {
    setError("");
    setMessage("");
    try {
      const body = new FormData();
      body.append("rating", String(Number(rating)));
      body.append("comment", comment);
      if (reviewPhoto) {
        body.append("review_photo", reviewPhoto);
      }
      await apiMultipart(`/api/bookings/${item.booking_id || item.id}/review`, token, body);
      setMessage("Review saved.");
      setReviewPhoto(null);
      onSaved();
    } catch (err) {
      setError(err.message);
    }
  };

  if (item.review_id) {
    return (
      <div className="reviewForm readOnlyReview">
        <div className="sectionTitleRow">
          <strong>Your review</strong>
          <span>{renderStars(item.review_rating || rating)}</span>
        </div>
        <p>{item.review_comment || comment || "No comment."}</p>
        {item.review_photo_url && <img className="reviewPhoto" src={apiURL(item.review_photo_url)} alt="" />}
      </div>
    );
  }

  return (
    <div className="reviewForm">
      <div className="sectionTitleRow">
        <strong>Rate the worker</strong>
        <span>{renderStars(rating)}</span>
      </div>
      <div className="starPicker" role="group" aria-label="Worker rating">
        {[1, 2, 3, 4, 5].map((value) => (
          <button
            key={value}
            type="button"
            className={value <= rating ? "active" : ""}
            onClick={() => setRating(value)}
            aria-label={`${value} stars`}
          >
            ★
          </button>
        ))}
      </div>
      <textarea value={comment} onChange={(event) => setComment(event.target.value)} placeholder="Write what went well and what could be better..." />
      <label className="fileButton fitButton">
        Attach photo
        <input type="file" accept="image/png,image/jpeg,image/webp" onChange={(event) => setReviewPhoto(event.target.files?.[0] || null)} />
      </label>
      {reviewPhoto && <span className="muted">{reviewPhoto.name}</span>}
      <button className="secondaryButton" type="button" onClick={saveReview}>
        Send review
      </button>
      <Messages message={message} error={error} />
    </div>
  );
}

function ChatPanel({ token, role }) {
  const [chats, setChats] = useState([]);
  const [activeChatID, setActiveChatID] = useState("");
  const [messages, setMessages] = useState([]);
  const [bookings, setBookings] = useState([]);
  const [content, setContent] = useState("");
  const [attachment, setAttachment] = useState(null);
  const [priceDraft, setPriceDraft] = useState("");
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const currentUserID = useMemo(() => tokenUserID(token), [token]);
  const activeChat = useMemo(
    () => chats.find((chat) => String(chat.chat_id) === String(activeChatID)),
    [activeChatID, chats],
  );
  const activeBooking = useMemo(
    () => bookings.find((booking) => String(booking.booking_id || booking.id) === String(activeChat?.booking_id)),
    [activeChat?.booking_id, bookings],
  );
  const activeBookingStatus = String(activeBooking?.status || activeBooking?.booking_status || "").toLowerCase();
  const activeBookingPrice = parseMoney(activeBooking?.final_price || 0);
  const messagesEndRef = useRef(null);

  const loadChats = useCallback(() => {
    setError("");
    apiGet("/api/chats", token)
      .then((data) => {
        const next = Array.isArray(data) ? data : data.chats || [];
        setChats(next);
        const storedID = localStorage.getItem("workers_marketplace_active_chat");
        const selectedID = storedID && next.some((chat) => String(chat.chat_id) === storedID)
          ? storedID
          : String(next[0]?.chat_id || "");
        setActiveChatID((current) => current || selectedID);
      })
      .catch((err) => setError(err.message));
  }, [token]);

  const loadBookings = useCallback(() => {
    apiGet("/api/bookings/my", token)
      .then((data) => setBookings(Array.isArray(data) ? data : data.bookings || []))
      .catch(() => setBookings([]));
  }, [token]);

  const loadMessages = useCallback((chatID) => {
    if (!chatID) {
      setMessages([]);
      return;
    }
    apiGet(`/api/chats/${chatID}/messages`, token)
      .then((data) => setMessages(Array.isArray(data) ? data : data.messages || []))
      .then(() => apiPatch(`/api/chats/${chatID}/read`, token, {}))
      .then(() => loadChats())
      .catch((err) => setError(err.message));
  }, [loadChats, token]);

  useEffect(() => {
    loadChats();
    loadBookings();
  }, [loadBookings, loadChats]);

  useEffect(() => {
    loadMessages(activeChatID);
    if (activeChatID) {
      localStorage.setItem("workers_marketplace_active_chat", String(activeChatID));
    }
  }, [activeChatID, loadMessages]);

  useEffect(() => {
    if (!activeChatID) return undefined;
    const socket = new WebSocket(wsURL(`/api/chats/${activeChatID}/ws`, token));
    socket.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data);
        if (payload.type === "message.created" && payload.message) {
          setMessages((current) => [...current.filter((msg) => msg.message_id !== payload.message.message_id), payload.message]);
          loadBookings();
          loadChats();
        }
      } catch {
        // Ignore malformed realtime payloads; REST refresh still keeps chat usable.
      }
    };
    socket.onerror = () => {
      // REST polling below keeps chat usable when WebSocket is unavailable on deploy.
    };
    return () => socket.close();
  }, [activeChatID, loadBookings, loadChats, token]);

  useEffect(() => {
    if (!activeChatID) return undefined;
    const intervalID = window.setInterval(() => {
      loadMessages(activeChatID);
      loadChats();
      loadBookings();
    }, 4000);
    return () => window.clearInterval(intervalID);
  }, [activeChatID, loadBookings, loadChats, loadMessages]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [messages]);

  const postChatText = async (text) => {
    if (!activeChatID) return null;
    const sent = await apiPost(`/api/chats/${activeChatID}/messages`, token, { content: text });
    setMessages((current) => [...current.filter((msg) => msg.message_id !== sent.message_id), sent]);
    return sent;
  };

  const send = async (event) => {
    event.preventDefault();
    if (!activeChatID) return;
    setError("");
    setMessage("");
    try {
      let sent;
      if (attachment) {
        const body = new FormData();
        body.append("content", content);
        body.append("attachment", attachment);
        sent = await apiMultipart(`/api/chats/${activeChatID}/messages`, token, body);
      } else {
        sent = await apiPost(`/api/chats/${activeChatID}/messages`, token, { content });
      }
      setMessages((current) => [...current.filter((msg) => msg.message_id !== sent.message_id), sent]);
      setContent("");
      setAttachment(null);
      setMessage("Message sent.");
      loadChats();
    } catch (err) {
      setError(err.message);
    }
  };

  const setBookingPrice = async (event) => {
    event.preventDefault();
    if (!activeChat?.booking_id) return;
    const amount = Number(String(priceDraft).replace(/\s/g, "").replace(",", "."));
    if (!Number.isFinite(amount) || amount <= 0) {
      setError("Enter a positive booking price.");
      return;
    }
    setError("");
    setMessage("");
    try {
      await apiPatch(`/api/bookings/${activeChat.booking_id}/price`, token, { final_price: amount });
      const priceText = `Booking price set: ${Math.round(amount).toLocaleString("ru-RU")} KZT`;
      await postChatText(priceText);
      loadBookings();
      setPriceDraft("");
      setMessage("Price sent. Waiting for customer confirmation.");
    } catch (err) {
      setError(err.message);
    }
  };

  const acceptPrice = async () => {
    if (!activeChat?.booking_id) return;
    const priceText = activeBookingPrice > 0 ? `${formatMoney(activeBookingPrice)} KZT` : "this price";
    if (!window.confirm(`Are you sure you want to accept ${priceText}?`)) {
      return;
    }
    setError("");
    setMessage("");
    try {
      await apiPatch(`/api/bookings/${activeChat.booking_id}/price/accept`, token, {});
      await postChatText(`Price accepted: ${priceText}`);
      loadBookings();
      loadChats();
      setMessage("Price accepted. Worker selected and booking is active.");
    } catch (err) {
      setError(err.message);
    }
  };

  const rejectPrice = async () => {
    if (!activeChat?.booking_id) return;
    setError("");
    setMessage("");
    try {
      await apiPatch(`/api/bookings/${activeChat.booking_id}/price/reject`, token, {});
      await postChatText("Price rejected.");
      loadBookings();
      loadChats();
      setMessage("Price rejected. The worker can send a new price in this chat.");
    } catch (err) {
      setError(err.message);
    }
  };

  const canWorkerSetPrice = role === "worker" && (activeBookingStatus === "price_pending" || activeBookingStatus === "scheduled");
  const canCustomerDecidePrice = role === "customer" && activeBookingStatus === "price_pending";

  return (
    <section className="pagePanel chatPage">
      <SectionHeader title="Chat" text="Talk about details, timing and proof files." />
      <Messages message={message} error={error} />
      <div className="chatLayout">
        <aside className="chatList">
          {chats.length === 0 && <EmptyState title="No chats yet" text="Open a chat from a booking card." />}
          {chats.map((chat) => (
            <button
              key={chat.chat_id}
              className={String(activeChatID) === String(chat.chat_id) ? "active" : ""}
              type="button"
              onClick={() => setActiveChatID(String(chat.chat_id))}
            >
              <strong>Booking #{chat.booking_id}</strong>
              <span>{chat.unread_count ? `${chat.unread_count} unread` : chat.status}</span>
            </button>
          ))}
        </aside>
        <div className="chatConversation">
          {canWorkerSetPrice && (
            <form className="priceComposer" onSubmit={setBookingPrice}>
              <Field label="Booking price, KZT" light>
                <input value={priceDraft} onChange={(event) => setPriceDraft(event.target.value)} inputMode="numeric" placeholder="Set price in this chat" />
              </Field>
              <button disabled={!activeChatID}>Set price</button>
            </form>
          )}
          {canCustomerDecidePrice && (
            <div className="priceDecision">
              <div>
                <strong>{activeBookingPrice > 0 ? `${formatMoney(activeBookingPrice)} KZT` : "Waiting for worker price"}</strong>
                <span>{activeBookingPrice > 0 ? "Accept the price to select this worker and activate the booking." : "Discuss details in chat first."}</span>
              </div>
              {activeBookingPrice > 0 && (
                <div className="rowActions">
                  <button type="button" onClick={acceptPrice}>Accept price</button>
                  <button className="secondaryButton" type="button" onClick={rejectPrice}>Reject</button>
                </div>
              )}
            </div>
          )}
          <div className="messageList">
            {!activeChatID && <EmptyState title="Choose chat" text="Select a booking chat on the left." />}
            {messages.map((msg) => (
              <ChatBubble key={msg.message_id} msg={msg} own={Number(msg.sender_user_id) === Number(currentUserID)} />
            ))}
            <div ref={messagesEndRef} />
          </div>
          <form className="chatComposer" onSubmit={send}>
            <textarea value={content} onChange={(event) => setContent(event.target.value)} placeholder="Write a message..." />
            <label className="fileButton chatFileButton">
              Attach
              <input type="file" accept="image/png,image/jpeg,image/webp,video/mp4,video/quicktime,video/webm,application/pdf,.doc,.docx,.txt" onChange={(event) => setAttachment(event.target.files?.[0] || null)} />
            </label>
            <button disabled={!activeChatID || (!content.trim() && !attachment)}>Send</button>
            {attachment && <span className="muted">{attachment.name}</span>}
          </form>
        </div>
      </div>
    </section>
  );
}

function ChatBubble({ msg, own }) {
  return (
    <article className={own ? "chatBubble own" : "chatBubble"}>
      <small>{own ? "You" : "Partner"}</small>
      {msg.content && !(msg.attachment_url && msg.content === "Attachment") && <span>{msg.content}</span>}
      {msg.attachment_url && (
        <AttachmentPreview msg={msg} />
      )}
      <footer className="chatMeta">
        <time dateTime={msg.created_at || msg.createdAt || ""}>{formatChatTime(msg.created_at || msg.createdAt)}</time>
        {own && <span className={msg.read_at || msg.readAt ? "read" : ""}>{msg.read_at || msg.readAt ? "✓✓" : "✓"}</span>}
      </footer>
    </article>
  );
}

function AttachmentPreview({ msg }) {
  const url = apiURL(msg.attachment_url);
  const type = String(msg.attachment_type || "").toLowerCase();
  const name = msg.attachment_name || "Open attachment";
  const lowerURL = url.toLowerCase();
  if (type.startsWith("image/") || /\.(jpg|jpeg|png|webp)$/i.test(lowerURL)) {
    return (
      <a className="chatAttachment media" href={url} target="_blank" rel="noreferrer">
        <img src={url} alt={name} />
      </a>
    );
  }
  if (type.startsWith("video/") || /\.(mp4|webm|mov)$/i.test(lowerURL)) {
    return (
      <video className="chatAttachment video" controls preload="metadata">
        <source src={url} type={msg.attachment_type || undefined} />
      </video>
    );
  }
  return (
    <a className="chatAttachment file" href={url} target="_blank" rel="noreferrer">
      {name}
    </a>
  );
}

function NotificationsPanel({ token, onNavigate }) {
  const [items, setItems] = useState([]);
  const [error, setError] = useState("");

  const load = useCallback(() => {
    apiGet("/api/notifications", token).then((data) => setItems(Array.isArray(data) ? data : data.notifications || [])).catch((err) => setError(err.message));
  }, [token]);

  useEffect(() => {
    load();
    const intervalID = window.setInterval(load, 5000);
    window.addEventListener("wm-notifications-updated", load);
    return () => {
      window.clearInterval(intervalID);
      window.removeEventListener("wm-notifications-updated", load);
    };
  }, [load]);

  const markAll = async () => {
    await apiPatch("/api/notifications/read-all", token, {});
    load();
  };

  const openAction = async (item) => {
    setError("");
    try {
      if (item.action_type === "booking_chat" && item.action_ref) {
        const chat = await apiPost("/api/chats", token, { booking_id: Number(item.action_ref) });
        if (chat?.chat_id) {
          localStorage.setItem("workers_marketplace_active_chat", String(chat.chat_id));
        }
        onNavigate?.("chats");
      } else if (item.action_type === "chat" && item.action_ref) {
        localStorage.setItem("workers_marketplace_active_chat", String(item.action_ref));
        onNavigate?.("chats");
      }
      const id = item.notification_id || item.id;
      if (id) {
        await apiPatch(`/api/notifications/${id}/read`, token, {}).catch(() => {});
      }
      load();
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="pagePanel">
      <SectionHeader title="Notifications" text="System messages and booking updates." />
      <button className="secondaryButton fitButton" onClick={markAll}>Mark all read</button>
      <Messages error={error} />
      <div className="dataList">
        {items.length === 0 && <EmptyState title="No notifications" text="New alerts will appear here." />}
        {items.map((item) => (
          <article className="dataRow" key={item.notification_id || item.id}>
            <strong>{item.title || item.type || "Notification"}</strong>
            <span>{item.message || item.body || ""}</span>
            {item.action_type && (
              <button className="secondaryButton fitButton" type="button" onClick={() => openAction(item)}>
                {item.action_label || "Open"}
              </button>
            )}
          </article>
        ))}
      </div>
    </section>
  );
}

function useNotificationFeed(token) {
  const [toastNotifications, setToastNotifications] = useState([]);
  const seenRef = useRef(new Set());
  const timersRef = useRef(new Map());

  const dismissToastNotification = useCallback((id) => {
    const timer = timersRef.current.get(id);
    if (timer) {
      window.clearTimeout(timer);
      timersRef.current.delete(id);
    }
    setToastNotifications((current) => current.filter((item) => notificationID(item) !== id));
  }, []);

  useEffect(() => {
    if (!token) {
      seenRef.current = new Set();
      setToastNotifications([]);
      return undefined;
    }

    let cancelled = false;
    const load = async () => {
      try {
        const data = await apiGet("/api/notifications?limit=10&unread=true", token);
        const items = Array.isArray(data) ? data : data.notifications || [];
        const fresh = items.filter((item) => {
          const id = notificationID(item);
          if (!id || seenRef.current.has(id)) {
            return false;
          }
          seenRef.current.add(id);
          return true;
        });
        if (!cancelled && fresh.length > 0) {
          setToastNotifications((current) => [...fresh, ...current].slice(0, 5));
          fresh.forEach((item) => {
            const id = notificationID(item);
            if (!id || timersRef.current.has(id)) {
              return;
            }
            const timer = window.setTimeout(() => dismissToastNotification(id), 15000);
            timersRef.current.set(id, timer);
          });
          window.dispatchEvent(new CustomEvent("wm-notifications-updated"));
        }
      } catch {
        // Toast polling must never break the main interface.
      }
    };

    load();
    const intervalID = window.setInterval(load, 5000);
    return () => {
      cancelled = true;
      window.clearInterval(intervalID);
      timersRef.current.forEach((timer) => window.clearTimeout(timer));
      timersRef.current.clear();
    };
  }, [dismissToastNotification, token]);

  return { toastNotifications, dismissToastNotification };
}

function NotificationToasts({ items, onDismiss, onAction }) {
  if (!items.length) {
    return null;
  }
  return (
    <div className="notificationToastStack" aria-live="polite">
      {items.map((item) => {
        const id = notificationID(item);
        return (
          <article className="notificationToast" key={id}>
            <button type="button" aria-label="Dismiss notification" onClick={() => onDismiss(id)}>x</button>
            <strong>{item.title || item.type || "Notification"}</strong>
            <span>{item.message || item.body || ""}</span>
            {item.action_type && (
              <button className="notificationToastAction" type="button" onClick={() => onAction?.(item)}>
                {item.action_label || "Open"}
              </button>
            )}
          </article>
        );
      })}
    </div>
  );
}

function notificationID(item) {
  return item?.notification_id || item?.id || `${item?.type || "notification"}-${item?.created_at || item?.title || ""}`;
}

function CustomerProfilePanel({ token, onNavigate }) {
  const { position, locate } = useGeolocation();
  const [profile, setProfile] = useState(null);
  const [form, setForm] = useState({ address: "", bio: "", latitude: "", longitude: "" });
  const [photo, setPhoto] = useState(null);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const loadProfile = useCallback(() => {
    setError("");
    apiGet("/api/customer/profile", token)
      .then((data) => {
        setProfile(data);
        setForm({
          address: data.address || "",
          bio: data.bio || "",
          latitude: data.latitude ? String(data.latitude) : "",
          longitude: data.longitude ? String(data.longitude) : "",
        });
      })
      .catch(() => {});
  }, [token]);

  useEffect(() => {
    loadProfile();
  }, [loadProfile]);

  const useCurrentLocation = async () => {
    const nextPosition = await locate();
    if (nextPosition) {
      setForm((current) => ({
        ...current,
        latitude: String(nextPosition.latitude),
        longitude: String(nextPosition.longitude),
      }));
      setForm((current) => ({ ...current, address: current.address || "Resolving address..." }));
      const nextAddress = await reverseGeocode(nextPosition);
      setForm((current) => ({ ...current, address: nextAddress }));
    }
  };

  const submit = async (event) => {
    event.preventDefault();
    setError("");
    setMessage("");
    try {
      const body = new FormData();
      body.append("address", form.address);
      body.append("bio", form.bio);
      if (form.latitude) body.append("latitude", form.latitude);
      if (form.longitude) body.append("longitude", form.longitude);
      if (photo) body.append("profile_photo", photo);
      const updated = await apiMultipart("/api/customer/profile", token, body);
      setProfile((current) => ({ ...(current || {}), ...updated }));
      setPhoto(null);
      setMessage("Profile saved.");
    } catch (err) {
      setError(err.message);
    }
  };

  const photoURL = profile?.profile_photo_url ? apiURL(profile.profile_photo_url) : "";

  return (
    <section className="pagePanel workerProfilePage">
      <SectionHeader title="Customer profile" text="Photo, address and booking preferences." />
      <div className="workerProfileHero">
        <div className="profilePhotoBlock">
          <div className="profilePhoto">
            <span>WM</span>
            {photoURL ? <img src={photoURL} alt="" onError={(event) => event.currentTarget.remove()} /> : null}
          </div>
          <label className="fileButton">
            Upload photo
            <input type="file" accept="image/png,image/jpeg,image/webp" onChange={(e) => setPhoto(e.target.files?.[0] || null)} />
          </label>
          {photo && <span className="muted">{photo.name}</span>}
        </div>
        <form className="profileEditForm" onSubmit={submit}>
          <Field label="About me" light>
            <textarea value={form.bio} onChange={(e) => setForm({ ...form, bio: e.target.value })} placeholder="Add notes for workers: entrance, preferred contact, timing..." />
          </Field>
          <Field label="Saved address" light>
            <input value={form.address} onChange={(e) => setForm({ ...form, address: e.target.value })} placeholder="Street, building, entrance" />
          </Field>
          <div className="rowActions">
            <button type="button" className="secondaryButton" onClick={useCurrentLocation}>Use current location</button>
            <button>Save profile</button>
          </div>
        </form>
      </div>
      <div className="profileLinks">
        <button className="profileLinkCard" type="button" onClick={() => onNavigate("bookings")}>
          <strong>My bookings</strong>
          <span>Open all customer bookings</span>
        </button>
        <button className="profileLinkCard" type="button" onClick={() => onNavigate("requests")}>
          <strong>My requests</strong>
          <span>Track created service requests</span>
        </button>
        <button className="profileLinkCard" type="button" onClick={() => onNavigate("find")}>
          <strong>Find worker</strong>
          <span>Back to map search</span>
        </button>
      </div>
      <PaymentMethodPanel token={token} />
      <Messages message={message} error={error} />
    </section>
  );
}

function WorkerProfilePanel({ token, onNavigate }) {
  const [profile, setProfile] = useState(null);
  const [bookings, setBookings] = useState([]);
  const [form, setForm] = useState({ bio: "" });
  const [photo, setPhoto] = useState(null);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const loadProfile = useCallback(() => {
    setError("");
    apiGet("/api/worker/profile", token)
      .then((data) => {
        setProfile(data);
        setForm({
          bio: data.bio || "",
        });
      })
      .catch((err) => setError(err.message));
  }, [token]);

  useEffect(() => {
    loadProfile();
    apiGet("/api/bookings/my", token)
      .then((data) => setBookings(Array.isArray(data) ? data : data.bookings || []))
      .catch(() => setBookings([]));
  }, [loadProfile, token]);

  const stats = useMemo(() => buildIncomeStats(bookings), [bookings]);

  const submit = async (event) => {
    event.preventDefault();
    setError("");
    setMessage("");
    try {
      const body = new FormData();
      body.append("bio", form.bio);
      if (photo) {
        body.append("profile_photo", photo);
      }
      const updated = await apiMultipart("/api/worker/profile", token, body);
      setProfile((current) => ({ ...(current || {}), ...updated }));
      setPhoto(null);
      setMessage("Profile saved.");
    } catch (err) {
      setError(err.message);
    }
  };

  const photoURL = profile?.profile_photo_url ? apiURL(profile.profile_photo_url) : "";

  return (
    <section className="pagePanel workerProfilePage">
      <SectionHeader title="Worker profile" text="Profile photo, bio, verified skills and income analytics." />
      <div className="workerProfileHero">
        <div className="profilePhotoBlock">
          <div className="profilePhoto">
            <span>WM</span>
            {photoURL ? <img src={photoURL} alt="" onError={(event) => event.currentTarget.remove()} /> : null}
          </div>
          <label className="fileButton">
            Upload photo
            <input type="file" accept="image/png,image/jpeg,image/webp" onChange={(e) => setPhoto(e.target.files?.[0] || null)} />
          </label>
          {photo && <span className="muted">{photo.name}</span>}
        </div>
        <form className="profileEditForm" onSubmit={submit}>
          <Field label="About me" light>
            <textarea value={form.bio} onChange={(e) => setForm({ ...form, bio: e.target.value })} placeholder="Tell customers about your experience, approach and city." />
          </Field>
          <button>Save profile</button>
        </form>
      </div>
      <div className="profileStatsGrid">
        <StatCard title="This week" value={formatMoney(stats.weekTotal) + " KZT"} text={stats.weekCompleted + " completed jobs"} />
        <StatCard title="This month" value={formatMoney(stats.monthTotal) + " KZT"} text={stats.monthCompleted + " completed jobs"} />
        <StatCard title="Rating" value={renderStars(profile?.rating || 0)} text={`${Number(profile?.rating || 0).toFixed(1)} average from customer reviews`} />
        <StatCard title="Average check" value={formatMoney(stats.average) + " KZT"} text="Completed jobs this month" />
      </div>
      <section className="profileSection">
        <div className="sectionTitleRow">
          <h3>Monthly income</h3>
          <span>{stats.monthCompleted} jobs</span>
        </div>
        <div className="incomeBars">
          {stats.weekBuckets.map((bucket) => (
            <div className="incomeBar" key={bucket.label}>
              <span>{bucket.label}</span>
              <div><b style={{ width: bucket.percent + "%" }} /></div>
              <strong>{formatMoney(bucket.total)}</strong>
            </div>
          ))}
        </div>
      </section>
      <section className="profileSection">
        <div className="sectionTitleRow">
          <h3>Verified skills</h3>
          <button className="secondaryButton fitButton" onClick={() => onNavigate("skills")}>Add service</button>
        </div>
        <div className="verifiedSkillsGrid">
          {(profile?.verified_skills || []).length === 0 && <EmptyState title="No verified skills yet" text="Add a service and attach qualification evidence." />}
          {(profile?.verified_skills || []).map((skill) => (
            <article className="verifiedSkillCard" key={skill.worker_skill_id}>
              <strong>{categoryTitle(skill.category_name)}</strong>
              <span>{skill.experience_level} - price agreed in chat</span>
            </article>
          ))}
        </div>
      </section>
      <div className="profileLinks">
        <button className="profileLinkCard" type="button" onClick={() => onNavigate("jobs")}>
          <strong>My jobs</strong>
          <span>Open assigned bookings</span>
        </button>
        <button className="profileLinkCard" type="button" onClick={() => onNavigate("pro")}>
          <strong>Map</strong>
          <span>Return to online mode and job search</span>
        </button>
        <button className="profileLinkCard" type="button" onClick={() => onNavigate("skills")}>
          <strong>Services</strong>
          <span>Manage verified skills</span>
        </button>
      </div>
      <PaymentMethodPanel token={token} />
      <Messages message={message} error={error} />
    </section>
  );
}

function PaymentMethodPanel({ token, onLinked, compact = false }) {
  const [method, setMethod] = useState(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(() => {
    setError("");
    apiGet("/api/payment-method", token)
      .then((nextMethod) => setMethod({
        ...nextMethod,
        last4: nextMethod?.last4 || nextMethod?.card_last4 || "",
      }))
      .catch((err) => setError(err.message));
  }, [token]);

  useEffect(() => load(), [load]);

  useEffect(() => {
    const handleLinked = () => {
      load();
      onLinked?.();
    };
    window.addEventListener("wm-payment-method-linked", handleLinked);
    return () => window.removeEventListener("wm-payment-method-linked", handleLinked);
  }, [load, onLinked]);

  const startSetup = async () => {
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const result = await apiPost("/api/payment-method/stripe/setup-session", token, {});
      const setupURL = result?.payment_setup_url || result?.url;
      if (!setupURL) {
        throw new Error("Payment setup URL is missing.");
      }
      window.location.href = setupURL;
    } catch (err) {
      setError(err.message);
      setBusy(false);
    }
  };

  return (
    <section className={compact ? "paymentGateSection" : "profileSection"}>
      <div className="sectionTitleRow">
        <h3>Payment card</h3>
        {method?.has_payment_method && <span>{method.provider || "Stripe"} {method.last4 ? `•••• ${method.last4}` : "linked"}</span>}
      </div>
      <p className="muted">
        {method?.has_payment_method
          ? "Payment method is linked and ready for bookings."
          : "You will be redirected to Stripe to link a payment method securely."}
      </p>
      {!method?.has_payment_method && (
        <button type="button" onClick={startSetup} disabled={busy}>
          {busy ? "Opening Stripe..." : "Link with Stripe"}
        </button>
      )}
      <Messages message={message} error={error} />
    </section>
  );
}

function WorkerSkillsPanel({ token }) {
  const [categories, setCategories] = useState([]);
  const [profileID, setProfileID] = useState("");
  const [form, setForm] = useState({ category_id: "", experience_level: "junior", price: "", evidence_note: "" });
  const [files, setFiles] = useState([]);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    apiGet("/api/categories", token)
      .then((items) => {
        setCategories(items);
        if (items.length > 0) {
          setForm((current) => current.category_id ? current : { ...current, category_id: String(items[0].category_id) });
        }
      })
      .catch((err) => setError(err.message));

    apiGet("/api/worker/profile", token)
      .then((data) => setProfileID(data.worker_profile_id || ""))
      .catch(() => setProfileID(""));
  }, [token]);

  const submit = async (event) => {
    event.preventDefault();
    setError("");
    setMessage("");
    try {
      if (!profileID) {
        const createdProfile = await apiPost("/api/worker/profile", token, { bio: "" });
        setProfileID(createdProfile.worker_profile_id || createdProfile.id || "created");
      }
      const body = new FormData();
      body.append("category_id", form.category_id);
      body.append("experience_level", form.experience_level);
      body.append("price_base", form.price || "0");
      body.append("evidence_note", form.evidence_note);
      files.forEach((file) => body.append("evidence_files", file));
      await apiMultipart("/api/worker/skills", token, body);
      setFiles([]);
      setMessage("Service request sent. Admin will review qualification evidence.");
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="pagePanel skillsPage">
      <SectionHeader title="Services" text="Choose a category, level and attach qualification evidence. Price is agreed in chat for each booking." />
      <div className="skillStatusGrid">
        <article className="skillStatusCard">
          <span>Verification</span>
          <strong>Attach certificates, work photos, diplomas or portfolio files. Admin approval is required before going online.</strong>
        </article>
      </div>
      <form className="skillForm" onSubmit={submit}>
        <Field label="Service category" light>
          <select value={form.category_id} onChange={(e) => setForm({ ...form, category_id: e.target.value })} required>
            {categories.length === 0 && <option value="">Categories are not loaded</option>}
            {categories.map((category) => <option key={category.category_id} value={category.category_id}>{categoryTitle(category.name)}</option>)}
          </select>
        </Field>
        <div className="field light">
          <span>Level</span>
          <div className="segmentedControl">
            {["junior", "middle", "senior"].map((level) => (
              <button key={level} type="button" className={form.experience_level === level ? "active" : ""} onClick={() => setForm({ ...form, experience_level: level })}>{level}</button>
            ))}
          </div>
        </div>
        <button>Add service</button>
        <Field label="Qualification evidence" light>
          <input type="file" multiple accept="image/png,image/jpeg,image/webp,application/pdf" onChange={(e) => setFiles(Array.from(e.target.files || []))} />
        </Field>
        <Field label="Admin note" light>
          <textarea value={form.evidence_note} onChange={(e) => setForm({ ...form, evidence_note: e.target.value })} placeholder="Example: 3 years of experience, certificate attached, recent work photos..." />
        </Field>
        <div className="selectedFiles">
          {files.length === 0 ? <span>No files selected</span> : files.map((file) => <span key={file.name}>{file.name}</span>)}
        </div>
      </form>
      <div className="skillCategoryGrid">
        {categories.map((category) => (
          <article key={category.category_id} className={String(category.category_id) === String(form.category_id) ? "categoryTile active" : "categoryTile"} onClick={() => setForm({ ...form, category_id: String(category.category_id) })}>
            <strong>{categoryTitle(category.name)}</strong>
            <span>{categoryDescription(category.name, category.description)}</span>
          </article>
        ))}
      </div>
      <Messages message={message} error={error} />
    </section>
  );
}

function StatCard({ title, value, text }) {
  return (
    <article className="statCard">
      <span>{title}</span>
      <strong>{value}</strong>
      <small>{text}</small>
    </article>
  );
}

function buildIncomeStats(bookings) {
  const now = new Date();
  const weekStart = new Date(now);
  weekStart.setDate(now.getDate() - 7);
  const monthStart = new Date(now);
  monthStart.setDate(now.getDate() - 30);

  const completed = bookings.filter((item) => String(item.status || item.booking_status || "").toLowerCase() === "completed");
  const inWeek = completed.filter((item) => bookingDate(item) >= weekStart);
  const inMonth = completed.filter((item) => bookingDate(item) >= monthStart);
  const weekTotal = sumBookings(inWeek);
  const monthTotal = sumBookings(inMonth);
  const average = inMonth.length > 0 ? monthTotal / inMonth.length : 0;
  const weekBuckets = Array.from({ length: 4 }, (_, index) => {
    const end = new Date(now);
    end.setDate(now.getDate() - index * 7);
    const start = new Date(end);
    start.setDate(end.getDate() - 7);
    const total = sumBookings(completed.filter((item) => {
      const date = bookingDate(item);
      return date >= start && date <= end;
    }));
    return { label: `W${4 - index}`, total };
  }).reverse();
  const max = Math.max(...weekBuckets.map((bucket) => bucket.total), 1);

  return {
    weekTotal,
    monthTotal,
    average,
    weekCompleted: inWeek.length,
    monthCompleted: inMonth.length,
    weekBuckets: weekBuckets.map((bucket) => ({ ...bucket, percent: Math.max(6, Math.round((bucket.total / max) * 100)) })),
  };
}

function bookingDate(item) {
  return new Date(item.end_time || item.start_time || item.scheduled_time || item.created_at || Date.now());
}

function sumBookings(items) {
  return items.reduce((sum, item) => sum + parseMoney(item.final_price || item.amount || 0), 0);
}

function parseMoney(value) {
  const parsed = Number.parseFloat(String(value ?? "0").replace(",", "."));
  return Number.isFinite(parsed) ? parsed : 0;
}

function formatMoney(value) {
  return Math.round(value).toLocaleString("ru-RU");
}

function formatDistanceLabel(value) {
  const distance = Number(value);
  if (!Number.isFinite(distance)) {
    return "nearby";
  }
  if (distance >= 1000) {
    return `${(distance / 1000).toFixed(1)} km away`;
  }
  return `${Math.round(distance)} m away`;
}

const navigationSpeechState = {
  key: "",
  spokenAt: 0,
};

function announceNewScheduledBookings(bookings, knownRef) {
  if (!knownRef.current) {
    knownRef.current = new Set(
      (bookings || [])
        .filter((booking) => String(booking.status || booking.booking_status || "").toLowerCase() === "scheduled")
        .map((booking) => booking.booking_id || booking.id)
        .filter(Boolean),
    );
    return;
  }
  for (const booking of bookings || []) {
    const id = booking.booking_id || booking.id;
    const status = String(booking.status || booking.booking_status || "").toLowerCase();
    if (!id) {
      continue;
    }
    if (status === "scheduled" && !knownRef.current.has(id)) {
      knownRef.current.add(id);
      speakText("Клиент принял цену. Начните поездку.");
    } else if (status === "scheduled") {
      knownRef.current.add(id);
    }
  }
}

function announceNavigationHint(current, destination) {
  const meters = haversineMeters(current, destination);
  const bucket = meters < 35 ? "arrived" : String(Math.round(meters / 100) * 100);
  const now = Date.now();
  if (navigationSpeechState.key === bucket && now - navigationSpeechState.spokenAt < 45000) {
    return;
  }
  navigationSpeechState.key = bucket;
  navigationSpeechState.spokenAt = now;
  const text = meters < 35
    ? "Вы на месте."
    : `Двигайтесь прямо примерно ${formatNavigationDistance(meters)}.`;
  speakText(text);
}

function speakText(text) {
  if (!("speechSynthesis" in window)) {
    return;
  }
  const utterance = new SpeechSynthesisUtterance(text);
  utterance.lang = "ru-RU";
  utterance.rate = 0.95;
  window.speechSynthesis.cancel();
  window.speechSynthesis.speak(utterance);
}

function formatNavigationDistance(meters) {
  if (meters >= 1000) {
    return `${(meters / 1000).toFixed(1).replace(".", ",")} километра`;
  }
  return `${Math.max(50, Math.round(meters / 50) * 50)} метров`;
}

function haversineMeters(a, b) {
  const radius = 6371000;
  const lat1 = degreesToRadians(Number(a.latitude));
  const lat2 = degreesToRadians(Number(b.latitude));
  const deltaLat = degreesToRadians(Number(b.latitude) - Number(a.latitude));
  const deltaLon = degreesToRadians(Number(b.longitude) - Number(a.longitude));
  const halfChord = Math.sin(deltaLat / 2) ** 2 +
    Math.cos(lat1) * Math.cos(lat2) * Math.sin(deltaLon / 2) ** 2;
  return 2 * radius * Math.atan2(Math.sqrt(halfChord), Math.sqrt(1 - halfChord));
}

function degreesToRadians(value) {
  return value * Math.PI / 180;
}

function isDirectRoute(points) {
  return Array.isArray(points) && points.length <= 2;
}

function formatChatTime(value) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

function renderStars(value) {
  const rating = Math.round(Number(value) || 0);
  return "★★★★★".split("").map((star, index) => index < rating ? star : "☆").join("");
}

function tokenUserID(token) {
  try {
    const payload = token?.split(".")?.[1];
    if (!payload) return 0;
    const normalized = payload.replace(/-/g, "+").replace(/_/g, "/");
    const decoded = JSON.parse(window.atob(normalized.padEnd(Math.ceil(normalized.length / 4) * 4, "=")));
    return Number(decoded.user_id || decoded.userId || decoded.sub || 0);
  } catch {
    return 0;
  }
}

function formatDateTime(value) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toLocaleString("en-GB", {
    day: "2-digit",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function ProfileForm({ title, text, form, setForm, onSubmit, links = [], onNavigate }) {
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const submit = async (event) => {
    event.preventDefault();
    setError("");
    try {
      await onSubmit();
      setMessage("Profile saved.");
    } catch (err) {
      setError(err.message);
    }
  };
  return (
    <section className="pagePanel">
      <SectionHeader title={title} text={text} />
      <form className="formGrid" onSubmit={submit}>
        {Object.keys(form).map((key) => (
          <Field label={key.replaceAll("_", " ")} light key={key}>
            <input value={form[key]} onChange={(e) => setForm({ ...form, [key]: e.target.value })} />
          </Field>
        ))}
        <button>Save profile</button>
      </form>
      {links.length > 0 && (
        <div className="profileLinks">
          {links.map(([target, titleText, body]) => (
            <button className="profileLinkCard" key={target} type="button" onClick={() => onNavigate?.(target)}>
              <strong>{titleText}</strong>
              <span>{body}</span>
            </button>
          ))}
        </div>
      )}
      <Messages message={message} error={error} />
    </section>
  );
}

function ListPanel({ title, endpoint, token, empty }) {
  const [items, setItems] = useState([]);
  const [error, setError] = useState("");
  useEffect(() => {
    apiGet(endpoint, token).then((data) => setItems(Array.isArray(data) ? data : Object.values(data || {}).find(Array.isArray) || [])).catch((err) => setError(err.message));
  }, [endpoint, token]);
  return (
    <section className="pagePanel">
      <SectionHeader title={title} text="Your marketplace activity." />
      <Messages error={error} />
      <div className="dataList">
        {items.length === 0 && <EmptyState title={empty} text="Nothing to show yet." />}
        {items.map((item, index) => (
          <pre className="jsonBox" key={item.id || item.request_id || index}>{JSON.stringify(item, null, 2)}</pre>
        ))}
      </div>
    </section>
  );
}

function JobBoard({ bookings, onProgress, compact }) {
  return (
    <div className={compact ? "jobBoard compact" : "jobBoard"}>
      {bookings.length === 0 && <EmptyState title="No active jobs" text="When a customer books you, the job card appears here." />}
      {bookings.map((booking) => {
        const id = booking.booking_id || booking.id;
        const status = String(booking.status || booking.booking_status || "pending").toLowerCase();
        return (
          <article className="jobCard" key={id}>
            <div>
              <strong>Job #{id}</strong>
              <span>{booking.status || booking.booking_status || "pending"}</span>
            </div>
            <p>{booking.address || booking.description || "Customer task"}</p>
            {booking.completion_evidence && <p>Proof sent: {booking.completion_evidence}</p>}
            <div className="rowActions">
              {status === "scheduled" && <button onClick={() => onProgress(id, "start")}>Start route</button>}
              {status === "in_progress" && <button className="secondaryButton" onClick={() => onProgress(id, "complete")}>Send proof</button>}
              {status === "awaiting_confirmation" && <span className="lightStatus">Waiting for customer confirmation</span>}
            </div>
          </article>
        );
      })}
    </div>
  );
}

function SectionHeader({ title, text }) {
  return (
    <header className="resultsHeader">
      <div>
        <h2>{title}</h2>
        <p>{text}</p>
      </div>
    </header>
  );
}

function Field({ label, light, children }) {
  return (
    <label className={light ? "field light" : "field"}>
      <span>{label}</span>
      {children}
    </label>
  );
}

function Messages({ message, error }) {
  if (!message && !error) return null;
  return (
    <div className="messageStack">
      {message && <p className="softMessage">{message}</p>}
      {error && <p className="errorMessage">{error}</p>}
    </div>
  );
}

function StatusLine({ geoStatus, geoError }) {
  return (
    <div className="statusStack lightStatus">
      <span>Location: {geoStatus}</span>
      {geoError && <span>{geoError}</span>}
    </div>
  );
}

function EmptyState({ title, text }) {
  return (
    <div className="emptyState">
      <strong>{title}</strong>
      <p>{text}</p>
    </div>
  );
}

function decodeToken(token) {
  try {
    const payload = token?.split(".")[1];
    if (!payload) return {};
    const normalized = payload.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(payload.length / 4) * 4, "=");
    return JSON.parse(atob(normalized));
  } catch {
    return {};
  }
}

function readRole(token) {
  return decodeToken(token).role || "";
}

function isTokenExpired(token) {
  const exp = Number(decodeToken(token).exp || 0);
  return !token || !exp || exp * 1000 <= Date.now();
}
