import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
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
const TTS_VOICE_HINT = import.meta.env.VITE_TTS_VOICE_HINT || "";
const ROUTE_REFRESH_MS = 90000;
const ROUTE_REFRESH_DISTANCE_M = 200;
const STAFF_AVATAR_URL = "/staff-avatar.png";

const roleTabs = {
  customer: [
    ["find", "Search"],
    ["bookings", "Bookings"],
    ["chats", "Chat"],
    ["reports", "Reports"],
    ["profile", "Profile"],
    ["notifications", "Alerts"],
  ],
  worker: [
    ["pro", "Map"],
    ["jobs", "Jobs"],
    ["chats", "Chat"],
    ["reports", "Reports"],
    ["skills", "Services"],
    ["profile", "Profile"],
    ["notifications", "Alerts"],
  ],
  admin: [
    ["overview", "Dashboard"],
    ["verify", "Queue"],
    ["users", "Users"],
    ["reports", "Reports"],
    ["accounts", "Staff"],
    ["notifications", "Alerts"],
  ],
  manager: [
    ["overview", "Dashboard"],
    ["verify", "Queue"],
    ["users", "Users"],
    ["reports", "Reports"],
    ["notifications", "Alerts"],
  ],
};

const reportReasonLabels = {
  bad_quality: "Bad quality",
  no_show: "No show",
  rude_behavior: "Rude behavior",
  fake_evidence: "Fake evidence",
  payment_disagreement: "Payment disagreement",
  other: "Other",
};

const reportStatusLabels = {
  open: "Open",
  pending: "Pending",
  reviewing: "Reviewing",
  resolved: "Resolved",
  rejected: "Rejected",
  closed: "Closed",
};

function openFilePreview(file) {
  window.dispatchEvent(new CustomEvent("wm-file-preview", { detail: file }));
}

function makeAvatarDraft(file) {
  if (!file) return null;
  return {
    file,
    previewURL: URL.createObjectURL(file),
    x: 50,
    y: 50,
    zoom: 1,
  };
}

function revokeAvatarDraft(draft) {
  if (draft?.previewURL) {
    URL.revokeObjectURL(draft.previewURL);
  }
}

async function cropAvatarDraft(draft) {
  if (!draft?.file) return null;
  const image = new Image();
  image.src = draft.previewURL;
  await image.decode();

  const sourceSize = Math.min(image.naturalWidth, image.naturalHeight) / Number(draft.zoom || 1);
  const maxX = Math.max(0, image.naturalWidth - sourceSize);
  const maxY = Math.max(0, image.naturalHeight - sourceSize);
  const sourceX = maxX * (Number(draft.x || 50) / 100);
  const sourceY = maxY * (Number(draft.y || 50) / 100);

  const canvas = document.createElement("canvas");
  canvas.width = 512;
  canvas.height = 512;
  const context = canvas.getContext("2d");
  context.drawImage(image, sourceX, sourceY, sourceSize, sourceSize, 0, 0, canvas.width, canvas.height);

  const blob = await new Promise((resolve) => canvas.toBlob(resolve, "image/png", 0.95));
  if (!blob) return draft.file;
  return new File([blob], draft.file.name.replace(/\.[^.]+$/, "") + "-avatar.png", { type: "image/png" });
}

function AvatarCropper({ draft, onChange, onClose, onCancel }) {
  if (!draft) return null;

  const stageRef = useRef(null);
  const draggingRef = useRef(false);
  const update = (patch) => onChange({ ...draft, ...patch });
  const updatePositionFromPointer = (event) => {
    const rect = stageRef.current?.getBoundingClientRect();
    if (!rect) return;
    const nextX = Math.min(100, Math.max(0, ((event.clientX - rect.left) / rect.width) * 100));
    const nextY = Math.min(100, Math.max(0, ((event.clientY - rect.top) / rect.height) * 100));
    update({ x: Math.round(nextX), y: Math.round(nextY) });
  };
  const imageStyle = {
    objectPosition: `${draft.x}% ${draft.y}%`,
    transform: `scale(${draft.zoom})`,
    transformOrigin: `${draft.x}% ${draft.y}%`,
  };

  return createPortal((
    <div className="avatarEditorOverlay" role="dialog" aria-modal="true" aria-label="Edit image" onMouseDown={onClose}>
      <div className="avatarEditorModal" onMouseDown={(event) => event.stopPropagation()}>
        <header>
          <strong>Edit image</strong>
          <button type="button" aria-label="Cancel image upload" onClick={onCancel}>×</button>
        </header>
        <div
          className="avatarCropStage"
          ref={stageRef}
          onPointerDown={(event) => {
            draggingRef.current = true;
            event.currentTarget.setPointerCapture(event.pointerId);
            updatePositionFromPointer(event);
          }}
          onPointerMove={(event) => {
            if (draggingRef.current) updatePositionFromPointer(event);
          }}
          onPointerUp={() => {
            draggingRef.current = false;
          }}
          onPointerCancel={() => {
            draggingRef.current = false;
          }}
        >
          <img
            src={draft.previewURL}
            alt=""
            style={imageStyle}
            draggable="false"
            onDragStart={(event) => event.preventDefault()}
          />
          <div className="avatarCropRing" aria-hidden="true" />
        </div>
        <div className="avatarCropControls">
          <span aria-hidden="true">□</span>
          <input aria-label="Zoom" type="range" min="1" max="3" step="0.05" value={draft.zoom} onChange={(event) => update({ zoom: Number(event.target.value) })} />
          <span aria-hidden="true">▣</span>
        </div>
        <p className="avatarEditorHint">Drag the photo to choose the area inside the circle.</p>
        <button className="avatarEditorApply" type="button" onClick={onClose}>Apply</button>
      </div>
    </div>
  ), document.body);
}

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
    } else if (item.action_type === "booking_map") {
      setActiveTab(role === "worker" ? "pro" : "find");
    } else if (item.action_type === "report" && item.action_ref) {
      localStorage.setItem("workers_marketplace_active_report", String(item.action_ref));
      setActiveTab("reports");
    } else if (item.action_type === "verify") {
      setActiveTab("verify");
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
        <FilePreviewPortal />
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
        <FilePreviewPortal />
      </>
    );
  }

  return (
    <>
      <AppFrame token={token} role={role} session={session} activeTab={activeTab} onTab={setActiveTab} onSignOut={signOut}>
        {(role === "admin" || role === "manager") && <AdminApp token={token} role={role} activeTab={activeTab} onNavigate={setActiveTab} />}
        {!["customer", "worker", "admin", "manager"].includes(role) && (
          <EmptyState title="Role is missing" text="Sign out and sign in again, or select a role in the backend." />
        )}
      </AppFrame>
      <NotificationToasts items={toastNotifications} onDismiss={dismissToastNotification} onAction={openToastAction} />
      <FilePreviewPortal />
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

  const updateRegisterPhone = (value) => {
    const trimmed = value.trimStart();
    const phone = trimmed === "" || trimmed.startsWith("+")
      ? trimmed
      : `+${trimmed.replace(/^\++/, "")}`;
    setRegister((current) => ({ ...current, phone }));
  };

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
            <Field label="Full name"><input value={register.full_name} onChange={(e) => setRegister({ ...register, full_name: e.target.value })} placeholder="Test User" required /></Field>
            <Field label="Email"><input value={register.email} onChange={(e) => setRegister({ ...register, email: e.target.value })} type="email" placeholder="test@example.com" required /></Field>
            <Field label="Phone"><input value={register.phone} onChange={(e) => updateRegisterPhone(e.target.value)} type="tel" inputMode="tel" pattern="^\+[0-9]{7,15}$" title="Phone must start with + and contain 7-15 digits." placeholder="+77001234567" required /></Field>
            <Field label="Role">
              <select value={register.role} onChange={(e) => setRegister({ ...register, role: e.target.value })}>
                <option value="customer">Customer</option>
                <option value="worker">Worker</option>
              </select>
            </Field>
            <Field label="Password"><input value={register.password} onChange={(e) => setRegister({ ...register, password: e.target.value })} type="password" placeholder="StrongPass123" required /></Field>
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

function nextSkillLevels(level) {
  switch (String(level || "").toLowerCase()) {
    case "junior":
      return ["middle", "senior"];
    case "middle":
      return ["senior"];
    default:
      return [];
  }
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
      output: "detailed",
      locale: "ru",
    },
  ];
  try {
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
    return null;
  } catch {
    return null;
  }
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
    collectRouteGeometry(route, add, (point) => points.push(point));
  }

  return dedupeRoutePoints(points);
}

function collectRouteGeometry(value, add, addPoint) {
  if (!value) {
    return;
  }
  if (typeof value === "string") {
    add(value);
    return;
  }
  if (Array.isArray(value)) {
    const point = parseCoordinatePair(value);
    if (point) {
      addPoint(point);
      return;
    }
    value.forEach((item) => collectRouteGeometry(item, add, addPoint));
    return;
  }
  if (typeof value === "object") {
    Object.values(value).forEach((item) => collectRouteGeometry(item, add, addPoint));
  }
}

function parseCoordinatePair(value) {
  if (!Array.isArray(value) || value.length < 2) {
    return null;
  }
  const longitude = Number(value[0]);
  const latitude = Number(value[1]);
  if (!Number.isFinite(latitude) || !Number.isFinite(longitude)) {
    return null;
  }
  if (Math.abs(latitude) > 90 || Math.abs(longitude) > 180) {
    return null;
  }
  return { latitude, longitude };
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

function AppFrame({ token, role, session, activeTab, onTab, onSignOut, children }) {
  const tabs = roleTabs[role] || [];
  const isStaffFrame = role === "admin" || role === "manager";
  const [adminNavData, setAdminNavData] = useState({ overview: null, users: [], reports: [] });

  useEffect(() => {
    if (!isStaffFrame || !token) {
      return undefined;
    }
    let cancelled = false;
    const loadAdminNavData = async () => {
      const [overviewResult, usersResult, reportsResult] = await Promise.allSettled([
        apiGet("/api/admin/overview", token),
        apiGet("/api/admin/users", token),
        apiGet("/api/reports", token),
      ]);
      if (cancelled) {
        return;
      }
      setAdminNavData((current) => ({
        overview: overviewResult.status === "fulfilled" ? overviewResult.value : current.overview,
        users: usersResult.status === "fulfilled" ? usersResult.value : current.users,
        reports: reportsResult.status === "fulfilled" ? reportsResult.value : current.reports,
      }));
    };
    loadAdminNavData();
    window.addEventListener("wm-admin-data-updated", loadAdminNavData);
    return () => {
      cancelled = true;
      window.removeEventListener("wm-admin-data-updated", loadAdminNavData);
    };
  }, [activeTab, isStaffFrame, token]);

  const adminTabBadge = (id) => {
    if (!isStaffFrame) {
      return null;
    }
    const stats = adminNavData.overview?.stats || {};
    const pendingIdentities = adminNavData.overview?.pending_identities?.length || 0;
    const pendingSkills = adminNavData.overview?.pending_skills?.length || Number(stats.pending_worker_skills || 0);
    const pendingUpgrades = adminNavData.overview?.pending_skill_upgrades?.length || 0;
    if (id === "verify") return pendingIdentities + pendingSkills + pendingUpgrades;
    if (id === "users") return Number(stats.users_total || adminNavData.users.length || 0);
    if (id === "reports") return adminNavData.reports.filter((report) => isOpenReportStatus(report.status)).length;
    if (id === "accounts") return adminNavData.users.filter((user) => user.role === "admin" || user.role === "manager").length;
    return null;
  };

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
          {tabs.map(([id, label]) => {
            const badge = adminTabBadge(id);
            return (
            <button key={id} className={activeTab === id ? "active" : ""} onClick={() => onTab(id)}>
              <span>{label}</span>
              {badge !== null && <small className={badge > 0 ? "navBadge hasItems" : "navBadge"}>{badge}</small>}
            </button>
            );
          })}
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
  const customerRouteRequestRef = useRef(null);

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
  });
  const trackingWorker = activeCustomerBooking ? [{
    worker_id: activeCustomerBooking.worker_profile_id,
    full_name: activeCustomerBooking.counterparty_name || "Worker",
    latitude: activeCustomerBooking.worker_latitude,
    longitude: activeCustomerBooking.worker_longitude,
    distance_meters: 0,
  }] : null;
  const customerDestination = activeCustomerBooking?.latitude && activeCustomerBooking?.longitude
    ? { latitude: Number(activeCustomerBooking.latitude), longitude: Number(activeCustomerBooking.longitude) }
    : null;
  const searchPosition = locationMode === "map" || locationMode === "address" ? pickedPosition : position;
  const routeSourceWorker = trackingWorker?.[0] || null;
  const routeDestination = customerDestination;

  useEffect(() => {
    let cancelled = false;
    if (!routeSourceWorker?.latitude || !routeSourceWorker?.longitude || !routeDestination?.latitude || !routeDestination?.longitude) {
      setRoutePoints(null);
      customerRouteRequestRef.current = null;
      return () => {
        cancelled = true;
      };
    }
    const workerPosition = {
      latitude: Number(routeSourceWorker.latitude),
      longitude: Number(routeSourceWorker.longitude),
    };
    const routeID = activeCustomerBooking?.booking_id || activeCustomerBooking?.id || `preview:${routeSourceWorker.worker_id || routeSourceWorker.worker_profile_id || ""}`;
    const routeKey = `${routeID}:${routeDestination.latitude}:${routeDestination.longitude}`;
    if (!shouldRefreshRoute(customerRouteRequestRef.current, routeKey, workerPosition)) {
      return () => {
        cancelled = true;
      };
    }
    buildDrivingRoute(workerPosition, routeDestination).then((points) => {
      if (!cancelled) {
        customerRouteRequestRef.current = { key: routeKey, start: workerPosition, at: Date.now() };
        setRoutePoints(points && points.length > 2 ? points : null);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [
    activeCustomerBooking?.booking_id,
    activeCustomerBooking?.id,
    routeSourceWorker?.worker_id,
    routeSourceWorker?.worker_profile_id,
    routeSourceWorker?.latitude,
    routeSourceWorker?.longitude,
    routeDestination?.latitude,
    routeDestination?.longitude,
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
      const nextWorkers = Array.isArray(data) ? data : data.workers || [];
      setWorkers(nextWorkers);
      setSelectedWorker((current) => current || nextWorkers[0] || null);
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

  if (activeTab === "requests") return <CustomerPhonePage activeTab="bookings" onNavigate={onNavigate} onSignOut={onSignOut}><BookingsPanel token={token} canConfirm onNavigate={onNavigate} showRequests /></CustomerPhonePage>;
  if (activeTab === "bookings") return <CustomerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><BookingsPanel token={token} canConfirm onNavigate={onNavigate} showRequests /></CustomerPhonePage>;
  if (activeTab === "chats") return <CustomerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><ChatPanel token={token} role="customer" /></CustomerPhonePage>;
  if (activeTab === "reports") return <CustomerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><ReportsPanel token={token} role="customer" /></CustomerPhonePage>;
  if (activeTab === "worker-profile") return <CustomerPhonePage activeTab="find" onNavigate={onNavigate} onSignOut={onSignOut}><WorkerPublicProfilePage token={token} worker={profileWorker} onBack={() => onNavigate("find")} onHireWorker={hireWorker} /></CustomerPhonePage>;
  if (activeTab === "profile") return <CustomerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><CustomerProfilePanel token={token} onNavigate={onNavigate} /></CustomerPhonePage>;
  if (activeTab === "notifications") return <CustomerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><NotificationsPanel token={token} onNavigate={onNavigate} /></CustomerPhonePage>;

  if (!position) {
    return <CustomerLocationGate activeTab={activeTab} geoStatus={geoStatus} geoError={geoError} onAllow={startWatch} onNavigate={onNavigate} onSignOut={onSignOut} />;
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
          routeFocusKey={activeCustomerBooking?.booking_id || activeCustomerBooking?.id || ""}
          navigationMode={Boolean(activeCustomerBooking)}
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
          </div>
          <div className="customerSearchPanel">
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
              <button className="customerSearchButton" onClick={searchWorkers} disabled={loading}>Search</button>
            </div>
            {locationMode === "map" && <p className="muted">Click on the map to choose the arrival point.</p>}
            <StatusLine geoStatus={geoStatus} geoError={geoError} />
            <Messages message={message} error={error} />
          </div>
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
  const innerClassName = activeTab === "chats" ? "customerInnerPage chatInnerPage" : "customerInnerPage";

  return (
    <div className="customerPhoneShell">
      <section className="customerPageScreen">
        <CustomerPhoneTabs activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut} />
        <div className={innerClassName}>{children}</div>
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

function CustomerLocationGate({ activeTab, geoStatus, geoError, onAllow, onNavigate, onSignOut }) {
  const loading = geoStatus === "loading";
  return (
    <main className="geoGate">
      <section className="geoGateCard">
        <div className="appIcon">WM</div>
        <h1>Allow location</h1>
        <p>We use your location to find nearby workers in Astana.</p>
        {loading && <div className="geoLoader" aria-hidden="true"><span /><span /></div>}
        <button onClick={onAllow} disabled={loading}>{loading ? "Finding location..." : "Allow location"}</button>
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
  const [followRoute, setFollowRoute] = useState(false);
  const knownScheduledBookingsRef = useRef(null);
  const workerRouteRequestRef = useRef(null);

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
    if (!position || !currentInProgressBooking) {
      setRoutePoints(null);
      workerRouteRequestRef.current = null;
      return () => {
        cancelled = true;
      };
    }
    if (!currentInProgressBooking.latitude || !currentInProgressBooking.longitude) {
      return () => {
        cancelled = true;
      };
    }
    const destination = {
      latitude: currentInProgressBooking.latitude,
      longitude: currentInProgressBooking.longitude,
    };
    const routeKey = `${currentInProgressBooking.booking_id || currentInProgressBooking.id || ""}:${destination.latitude}:${destination.longitude}`;
    if (!shouldRefreshRoute(workerRouteRequestRef.current, routeKey, position)) {
      return () => {
        cancelled = true;
      };
    }
    workerRouteRequestRef.current = { key: routeKey, start: position, at: Date.now() };
    buildDrivingRoute(position, destination).then((points) => {
      if (!cancelled) {
        setRoutePoints((current) => {
          if (!points || points.length < 2) {
            return current;
          }
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
    if (!currentInProgressBooking) {
      setFollowRoute(false);
    }
  }, [currentInProgressBooking?.booking_id]);

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
        await apiPatch("/api/worker/availability", token, { is_available: false }).catch(() => {});
        setAvailable(false);
        setSearching(false);
        setMessage("Proof photo sent. Waiting for customer confirmation.");
      } else {
        await apiPatch(`/api/bookings/${bookingID}/${action}`, token, {});
        setMessage(action === "reject" ? "Booking rejected." : `Booking ${action}ed.`);
        if (action === "start") {
          await apiPatch("/api/worker/availability", token, { is_available: false }).catch(() => {});
          setSearching(false);
          setAvailable(false);
          setFollowRoute(true);
          onNavigate("pro");
          window.setTimeout(() => mapRef.current?.follow?.(), 50);
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
  if (activeTab === "reports") {
    return <WorkerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><ReportsPanel token={token} role="worker" /></WorkerPhonePage>;
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
    return <WorkerLocationGate activeTab={activeTab} geoStatus={geoStatus} geoError={geoError} onAllow={startWatch} onNavigate={onNavigate} onSignOut={onSignOut} />;
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
          followPosition={followRoute && Boolean(currentInProgressBooking)}
          navigationMode={Boolean(currentInProgressBooking)}
        />
        <WorkerPhoneTabs activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut} />
        {currentInProgressBooking && (
          <NavigationOverlay
            booking={currentInProgressBooking}
            position={position}
            onFollow={() => {
              setFollowRoute(true);
              mapRef.current?.follow?.();
            }}
          />
        )}
        {available && searching && bookings.length === 0 && (
          <div className="searchPulse" aria-hidden="true">
            <span />
            <span />
            <span />
          </div>
        )}
        {!currentInProgressBooking && (
          <button className={available ? "searchButton lineSearchButton online" : "searchButton lineSearchButton"} onClick={toggleAvailability}>
            {available ? "Offline" : "Go online"}
          </button>
        )}
        <button className="roundMapButton plusButton" onClick={() => mapRef.current?.zoomIn()}>+</button>
        <button className="roundMapButton minusButton" onClick={() => mapRef.current?.zoomOut()}>-</button>
        <button
          className="roundMapButton navButtonMap"
          onClick={() => {
            if (currentInProgressBooking) {
              setFollowRoute(true);
              mapRef.current?.follow?.();
              return;
            }
            mapRef.current?.recenter();
          }}
        >
          GPS
        </button>
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
  const innerClassName = activeTab === "chats" ? "workerInnerPage chatInnerPage" : "workerInnerPage";

  return (
    <div className="proPhoneShell">
      <section className="proPhone workerPagePhone">
        <WorkerPhoneTabs activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut} />
        <div className={innerClassName}>{children}</div>
      </section>
    </div>
  );
}

function WorkerLocationGate({ activeTab, geoStatus, geoError, onAllow, onNavigate, onSignOut }) {
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

function NavigationOverlay({ booking, position, onFollow }) {
  const destination = {
    latitude: booking?.latitude,
    longitude: booking?.longitude,
  };
  const meters = position && destination.latitude && destination.longitude
    ? haversineMeters(position, destination)
    : 0;
  const primary = meters > 0 && meters < 35
    ? "Вы на месте"
    : meters > 0
      ? `Следуйте по маршруту ${formatNavigationDistance(meters)}`
      : "Следуйте по маршруту";
  return (
    <div className="navigationOverlay">
      <div>
        <strong>{primary}</strong>
        <span>{booking?.address || booking?.description || "Адрес клиента"}</span>
      </div>
      <button type="button" onClick={onFollow}>GPS</button>
    </div>
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
  const [reports, setReports] = useState([]);
  const [staffProfileUserID, setStaffProfileUserID] = useState("");
  const [staffForm, setStaffForm] = useState({ full_name: "", email: "", phone: "", password: "", role: "manager" });
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const loadOverview = useCallback(() => {
    apiGet("/api/admin/overview", token).then(setOverview).catch((err) => setError(err.message));
  }, [token]);
  const loadUsers = useCallback(() => {
    apiGet("/api/admin/users", token).then(setUsers).catch((err) => setError(err.message));
  }, [token]);
  const loadReports = useCallback(() => {
    apiGet("/api/reports", token).then((data) => setReports(Array.isArray(data) ? data : [])).catch(() => setReports([]));
  }, [token]);

  useEffect(() => {
    if (activeTab === "overview" || activeTab === "verify") {
      loadOverview();
    }
    if (activeTab === "users" || activeTab === "accounts" || activeTab === "overview") {
      loadUsers();
    }
    if (activeTab === "overview") {
      loadReports();
    }
  }, [activeTab, loadOverview, loadReports, loadUsers]);

  useEffect(() => {
    setStaffProfileUserID("");
  }, [activeTab]);

  const openStaffProfile = (userID) => {
    if (!userID) return;
    setStaffProfileUserID(String(userID));
  };

  if (staffProfileUserID) {
    return <StaffUserProfilePanel token={token} userID={staffProfileUserID} onBack={() => setStaffProfileUserID("")} />;
  }

  if (activeTab === "notifications") return <NotificationsPanel token={token} onNavigate={onNavigate} />;
  if (activeTab === "reports") return <ReportsPanel token={token} role={role} staff onOpenProfile={openStaffProfile} />;

  const verifySkill = async (id) => {
    setError("");
    setMessage("");
    try {
      await apiPost("/api/admin/verify-skill", token, { worker_skill_id: Number(id) });
      setMessage("Skill verified. Worker becomes verified after ID document is verified too.");
      loadOverview();
      window.dispatchEvent(new CustomEvent("wm-admin-data-updated"));
    } catch (err) {
      setError(err.message);
    }
  };

  const verifySkillUpgrade = async (id) => {
    setError("");
    setMessage("");
    try {
      await apiPost("/api/admin/verify-skill-upgrade", token, { upgrade_request_id: Number(id) });
      setMessage("Skill level upgraded.");
      loadOverview();
      window.dispatchEvent(new CustomEvent("wm-admin-data-updated"));
    } catch (err) {
      setError(err.message);
    }
  };

  const verifyIdentityDocument = async (id) => {
    setError("");
    setMessage("");
    try {
      await apiPost("/api/admin/verify-identity-document", token, { identity_document_id: Number(id) });
      setMessage("Identity document verified.");
      loadOverview();
      window.dispatchEvent(new CustomEvent("wm-admin-data-updated"));
    } catch (err) {
      setError(err.message);
    }
  };

  const assignIdentityDocument = async (id) => {
    setError("");
    setMessage("");
    try {
      await apiPost("/api/admin/assign-identity-document", token, { identity_document_id: Number(id) });
      setMessage("Identity document assigned to you.");
      loadOverview();
      window.dispatchEvent(new CustomEvent("wm-admin-data-updated"));
    } catch (err) {
      setError(err.message);
    }
  };

  const rejectIdentityDocument = async (id) => {
    const reason = window.prompt("Why should this identity document be rejected?");
    if (reason === null) {
      return;
    }
    setError("");
    setMessage("");
    try {
      await apiPost("/api/admin/reject-identity-document", token, {
        identity_document_id: Number(id),
        reason,
      });
      setMessage("Identity document rejected. Worker was notified.");
      loadOverview();
      window.dispatchEvent(new CustomEvent("wm-admin-data-updated"));
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
      window.dispatchEvent(new CustomEvent("wm-admin-data-updated"));
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
      window.dispatchEvent(new CustomEvent("wm-admin-data-updated"));
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
      window.dispatchEvent(new CustomEvent("wm-admin-data-updated"));
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
        <AdminOverviewPanel overview={overview} users={users} reports={reports} onNavigate={onNavigate} isAdmin={isAdmin} />
      )}
      {activeTab === "verify" && (
        <AdminVerificationPanel
          overview={overview}
          role={role}
          currentUserID={tokenUserID(token)}
          assignIdentityDocument={assignIdentityDocument}
          verifyIdentityDocument={verifyIdentityDocument}
          rejectIdentityDocument={rejectIdentityDocument}
          verifySkill={verifySkill}
          verifySkillUpgrade={verifySkillUpgrade}
        />
      )}
      {activeTab === "users" && <AdminUsersPanel users={users} onActivate={activateUser} onDelete={deleteUser} canDelete={isAdmin} onOpenProfile={openStaffProfile} />}
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

function AdminOverviewPanel({ overview, users, reports, onNavigate, isAdmin }) {
  const stats = overview?.stats || {};
  const pendingIdentities = overview?.pending_identities || [];
  const pendingSkills = overview?.pending_skills || [];
  const pendingUpgrades = overview?.pending_skill_upgrades || [];
  const openReports = reports.filter((report) => isOpenReportStatus(report.status));
  const staffCount = users.filter((user) => user.role === "admin" || user.role === "manager").length;
  const pendingQueueCount = pendingIdentities.length + pendingSkills.length + pendingUpgrades.length;

  return (
    <div className="adminOverview">
      <div className="adminKpiGrid">
        <AdminKpiCard title="Users" value={stats.users_total || users.length || 0} text={`${stats.customers_total || 0} customers, ${stats.workers_total || 0} workers`} />
        <AdminKpiCard title="Queue" value={pendingQueueCount} text={`${pendingIdentities.length} ID, ${pendingSkills.length} skills, ${pendingUpgrades.length} upgrades`} tone={pendingQueueCount > 0 ? "warning" : ""} />
        <AdminKpiCard title="Reports" value={openReports.length} text="Open support cases" tone={openReports.length > 0 ? "warning" : ""} />
        <AdminKpiCard title="Bookings" value={stats.bookings_total || 0} text={`${stats.bookings_in_progress || 0} in progress`} />
      </div>
      <section className="adminAttentionPanel">
        <div className="sectionTitleRow">
          <h3>Needs attention</h3>
          <span>{pendingQueueCount + openReports.length} open items</span>
        </div>
        <div className="adminActionGrid">
          <button className="adminActionCard" type="button" onClick={() => onNavigate("verify")}>
            <span className="adminActionIcon" aria-hidden="true">Q</span>
            <span>
              <strong>Review queue</strong>
              <small>{pendingQueueCount} verification items</small>
            </span>
            <b>Open</b>
          </button>
          <button className="adminActionCard" type="button" onClick={() => onNavigate("reports")}>
            <span className="adminActionIcon" aria-hidden="true">R</span>
            <span>
              <strong>Reports</strong>
              <small>{openReports.length} support cases need attention</small>
            </span>
            <b>Open</b>
          </button>
          <button className="adminActionCard" type="button" onClick={() => onNavigate("users")}>
            <span className="adminActionIcon" aria-hidden="true">U</span>
            <span>
              <strong>User support</strong>
              <small>Search, activate and review user accounts</small>
            </span>
            <b>Open</b>
          </button>
          {isAdmin && (
            <button className="adminActionCard" type="button" onClick={() => onNavigate("accounts")}>
              <span className="adminActionIcon" aria-hidden="true">S</span>
              <span>
                <strong>Staff access</strong>
                <small>{staffCount} admins and managers</small>
              </span>
              <b>Open</b>
            </button>
          )}
        </div>
      </section>
      <AdminMiniQueue title="Latest pending skills" items={pendingSkills} empty="No skill evidence to review." />
    </div>
  );
}

function StaffUserProfilePanel({ token, userID, onBack }) {
  const [profile, setProfile] = useState(null);
  const [error, setError] = useState("");

  useEffect(() => {
    setError("");
    setProfile(null);
    apiGet(`/api/admin/users/${userID}/profile`, token)
      .then(setProfile)
      .catch((err) => setError(err.message));
  }, [token, userID]);

  const user = profile?.user || {};
  const customer = profile?.customer_profile;
  const worker = profile?.worker_profile;
  const rawAvatarURL = customer?.profile_photo_url || worker?.profile_photo_url || "";
  const avatarURL = rawAvatarURL ? apiURL(rawAvatarURL) : "";
  const penalties = Array.isArray(profile?.penalties) ? profile.penalties : [];
  const identityDocuments = Array.isArray(profile?.identity_documents) ? profile.identity_documents : [];
  const activeWarnings = Number(profile?.warning_count || 0);

  return (
    <section className="adminWorkspace staffProfilePage">
      <button className="secondaryButton staffBackButton" type="button" onClick={onBack}>Back</button>
      <SectionHeader title="User profile" text="Staff-only account view with warnings, penalties and verified worker data." />
      <Messages error={error} />
      {!profile && !error && <EmptyState title="Loading profile" text="Fetching user details..." />}
      {profile && (
        <div className="staffProfileLayout">
          <section className="toolCard staffProfileHero">
            <div className="staffProfileAvatar">
              {avatarURL ? <img src={avatarURL} alt="" /> : initialsOf(user.full_name || user.email)}
            </div>
            <div>
              <span className={`rolePill ${user.role || ""}`}>{user.role}</span>
              <h2>{user.full_name || "User"}</h2>
              <p>{user.email}</p>
              <div className="staffProfileMeta">
                <span className={`statusPill ${String(user.status || "").toLowerCase()}`}>{user.status}</span>
                <span>{user.phone || "No phone"}</span>
                <span>Created {user.created_at || "-"}</span>
              </div>
            </div>
          </section>

          <section className={activeWarnings > 0 ? "toolCard staffWarningCard warning" : "toolCard staffWarningCard"}>
            <span>Warnings</span>
            <strong>{activeWarnings}</strong>
            <p>{activeWarnings > 0 ? "Active warning penalties are visible to staff here." : "No active warnings for this user."}</p>
          </section>

          {customer && (
            <section className="toolCard staffProfileSection">
              <h3>Customer details</h3>
              <p>{customer.bio || "No customer bio."}</p>
              <small>{customer.address || "No saved address."}</small>
            </section>
          )}

          {worker && (
            <section className="toolCard staffProfileSection">
              <h3>Worker details</h3>
              <div className="staffProfileMeta">
                <span>Rating {Number(worker.rating || 0).toFixed(1)}</span>
                <span>{worker.verification_status}</span>
                <span>{worker.is_available ? "Online" : "Offline"}</span>
              </div>
              <p>{worker.bio || "No worker bio."}</p>
              <div className="staffSkillGrid">
                {(profile.verified_skills || []).length === 0 && <AdminEmptyState title="No verified skills" text="Verified services will appear here." />}
                {(profile.verified_skills || []).map((skill) => (
                  <article key={skill.worker_skill_id}>
                    <strong>{categoryTitle(skill.category_name)}</strong>
                    <span>{skill.experience_level}</span>
                    <small>{formatMoney(parseMoney(skill.price_base))} KZT</small>
                  </article>
                ))}
              </div>
            </section>
          )}

          {worker && (
            <section className="toolCard staffProfileSection">
              <div className="sectionTitleRow">
                <h3>Identity documents</h3>
                <span>{identityDocuments.length}</span>
              </div>
              {identityDocuments.length === 0 && <AdminEmptyState title="No ID document" text="Worker has not uploaded an identity document yet." />}
              {identityDocuments.map((doc) => (
                <article className="staffPenaltyRow" key={doc.identity_document_id}>
                  <div>
                    <strong>{doc.file_name || "Identity document"}</strong>
                    <span>{doc.uploaded_at || "Uploaded"}</span>
                  </div>
                  <span className={`statusPill ${doc.status || ""}`}>{doc.status}</span>
                  <EvidenceLinks value={doc.file_path} />
                </article>
              ))}
            </section>
          )}

          <section className="toolCard staffProfileSection staffPenaltySection">
            <div className="sectionTitleRow">
              <h3>Penalties</h3>
              <span>{penalties.length}</span>
            </div>
            {penalties.length === 0 && <AdminEmptyState title="No penalties" text="Warnings and suspensions will appear here." />}
            {penalties.map((item) => (
              <article className="staffPenaltyRow" key={item.penalty_id}>
                <div>
                  <strong>{String(item.penalty_type || "").replaceAll("_", " ")}</strong>
                  <span>{item.reason || "No reason"}</span>
                </div>
                <span className={`statusPill ${item.status || ""}`}>{item.status}</span>
                <small>{item.report_id ? `Report #${item.report_id}` : "No report"}</small>
              </article>
            ))}
          </section>
        </div>
      )}
    </section>
  );
}

function AdminKpiCard({ title, value, text, tone = "" }) {
  return (
    <article className={tone ? `adminKpiCard ${tone}` : "adminKpiCard"}>
      <span>{title}</span>
      <strong>{value}</strong>
      <small>{text}</small>
    </article>
  );
}

function AdminMiniQueue({ title, items, empty }) {
  return (
    <section className="toolCard adminMiniQueue">
      <div className="sectionTitleRow">
        <h3>{title}</h3>
        <span>{items.length}</span>
      </div>
      <div className="dataList adminCompactList">
        {items.length === 0 && <AdminEmptyState title={empty} text="Everything is calm here." />}
        {items.slice(0, 5).map((item) => (
          <article className="adminCompactRow" key={item.worker_skill_id}>
            <div>
              <strong>{categoryTitle(item.category_name)}</strong>
              <span>{item.worker_full_name || item.worker_user_email}</span>
            </div>
            <span className="statusPill pending">Skill #{item.worker_skill_id}</span>
          </article>
        ))}
      </div>
    </section>
  );
}

function AdminEmptyState({ title, text }) {
  return (
    <div className="adminEmptyState">
      <strong>{title}</strong>
      <span>{text}</span>
    </div>
  );
}

function AdminUsersPanel({ users, onActivate, onDelete, canDelete, onOpenProfile }) {
  const [query, setQuery] = useState("");
  const [roleFilter, setRoleFilter] = useState("all");
  const queryText = query.trim().toLowerCase();
  const roleOptions = [
    ["all", "All", users.length],
    ["customer", "Customers", users.filter((user) => user.role === "customer").length],
    ["worker", "Workers", users.filter((user) => user.role === "worker").length],
    ["staff", "Staff", users.filter((user) => user.role === "admin" || user.role === "manager").length],
  ];
  const filteredUsers = users.filter((user) => {
    const matchesQuery = !queryText ||
      String(user.full_name || "").toLowerCase().includes(queryText) ||
      String(user.email || "").toLowerCase().includes(queryText);
    const matchesRole = roleFilter === "all" ||
      user.role === roleFilter ||
      (roleFilter === "staff" && (user.role === "admin" || user.role === "manager"));
    return matchesQuery && matchesRole;
  });

  return (
    <div className="adminUsersPanel">
      <div className="adminToolbar">
        <label className="adminSearchField">
          <span>Search users</span>
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Name or email" />
        </label>
        <div className="adminFilterTabs" aria-label="Filter users">
          {roleOptions.map(([value, label, count]) => (
            <button key={value} type="button" className={roleFilter === value ? "active" : ""} onClick={() => setRoleFilter(value)}>
              {label}
              <small>{count}</small>
            </button>
          ))}
        </div>
      </div>
      <div className="adminUserTable">
        <div className="adminUserTableHead">
          <span>User</span>
          <span>Role</span>
          <span>Status</span>
          <span>Actions</span>
        </div>
        {filteredUsers.length === 0 && <AdminEmptyState title="No users found" text="Try another search or filter." />}
        {filteredUsers.map((user) => (
          <article className="adminUserRow" key={user.user_id}>
            <button
              className="adminUserIdentity adminUserIdentityButton"
              type="button"
              disabled={!["customer", "worker"].includes(user.role)}
              onClick={() => onOpenProfile(user.user_id)}
              title={["customer", "worker"].includes(user.role) ? "Open profile" : ""}
            >
              <span className="adminUserAvatar">{initialsOf(user.full_name || user.email)}</span>
              <div>
                <strong>{user.full_name}</strong>
                <span>{user.email}</span>
              </div>
            </button>
            <span className={`rolePill ${user.role}`}>{user.role}</span>
            <span className={`statusPill ${String(user.status || "").toLowerCase()}`}>{user.status}</span>
            <div className="adminUserActions">
              {user.status !== "active" && <button type="button" onClick={() => onActivate(user.user_id)}>Activate</button>}
              {canDelete && <button type="button" className="dangerButton subtleDangerButton" onClick={() => onDelete(user.user_id)}>Delete</button>}
            </div>
          </article>
        ))}
      </div>
    </div>
  );
}

function AdminCreatePanel({ admins, managers, form, setForm, onSubmit, onDelete }) {
  const staffMembers = [...admins, ...managers].sort((a, b) => String(a.role).localeCompare(String(b.role)) || String(a.full_name).localeCompare(String(b.full_name)));

  return (
    <div className="adminStaffLayout">
      <form className="toolCard adminCreateForm adminStaffForm" onSubmit={onSubmit}>
        <div className="sectionTitleRow">
          <h3>Create staff account</h3>
          <span>{form.role}</span>
        </div>
        <p className="muted">Managers review support queues. Admins can also delete users and create staff accounts.</p>
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
      <section className="toolCard adminStaffPanel">
        <div className="sectionTitleRow">
          <h3>Current staff</h3>
          <span>{staffMembers.length} accounts</span>
        </div>
        <div className="adminStaffList">
          {staffMembers.length === 0 && <AdminEmptyState title="No staff accounts" text="Create a manager or admin account on the left." />}
          {staffMembers.map((staff) => (
            <article className="adminStaffRow" key={staff.user_id}>
              <div className="adminUserIdentity">
                <span className="adminUserAvatar">{initialsOf(staff.full_name || staff.email)}</span>
                <div>
                  <strong>{staff.full_name}</strong>
                  <span>{staff.email}</span>
                </div>
              </div>
              <span className={`rolePill ${staff.role}`}>{staff.role}</span>
              <button type="button" className="dangerButton subtleDangerButton" onClick={() => onDelete(staff.user_id)}>
                Delete
              </button>
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
  role,
  currentUserID,
  assignIdentityDocument,
  verifyIdentityDocument,
  rejectIdentityDocument,
  verifySkill,
  verifySkillUpgrade,
}) {
  const pendingIdentities = overview?.pending_identities || [];
  const visibleIdentities = role === "manager"
    ? pendingIdentities.filter((doc) => !doc.assigned_manager_id || Number(doc.assigned_manager_id) === Number(currentUserID))
    : pendingIdentities;
  const pendingSkills = overview?.pending_skills || [];
  const pendingUpgrades = overview?.pending_skill_upgrades || [];
  const [queueMode, setQueueMode] = useState("identity");
  const showingIdentity = queueMode === "identity";
  const showingSkills = queueMode === "skills";
  const showingUpgrades = queueMode === "upgrades";

  return (
    <div className="adminVerifyGrid">
      <section className="toolCard adminQueueCard">
        <div className="adminQueueHeader">
          <div>
            <h3>Review queue</h3>
            <p className="muted">Approve evidence before services become visible to customers.</p>
          </div>
          <div className="adminFilterTabs" aria-label="Queue type">
            <button type="button" className={showingIdentity ? "active" : ""} onClick={() => setQueueMode("identity")}>
              Identity
              <small>{visibleIdentities.length}</small>
            </button>
            <button type="button" className={showingSkills ? "active" : ""} onClick={() => setQueueMode("skills")}>
              New skills
              <small>{pendingSkills.length}</small>
            </button>
            <button type="button" className={showingUpgrades ? "active" : ""} onClick={() => setQueueMode("upgrades")}>
              Upgrades
              <small>{pendingUpgrades.length}</small>
            </button>
          </div>
        </div>
        <div className="dataList adminQueueList">
          {showingIdentity && visibleIdentities.length === 0 && <AdminEmptyState title="No pending ID documents" text="Worker identity documents will appear here." />}
          {showingIdentity && visibleIdentities.map((doc) => {
            const assignedToMe = Number(doc.assigned_manager_id) === Number(currentUserID);
            const unassigned = !doc.assigned_manager_id;
            const canReview = role === "admin" || assignedToMe;
            return (
            <article className="adminQueueRow" key={doc.identity_document_id}>
              <div>
                <strong>Identity document</strong>
                <span>{doc.worker_full_name} - {doc.worker_user_email}</span>
              </div>
              <div className="adminQueueMeta">
                <span>Worker #{doc.worker_profile_id}</span>
                <span>Document #{doc.identity_document_id}</span>
                {unassigned && <span className="statusPill pending">unassigned</span>}
                {assignedToMe && <span className="statusPill in_review">assigned to me</span>}
                {!unassigned && !assignedToMe && <span className="statusPill in_review">assigned #{doc.assigned_manager_id}</span>}
                {doc.created_at && <span>{formatDateTime(doc.created_at)}</span>}
              </div>
              <EvidenceLinks value={doc.file_path} />
              <div className="rowActions">
                {unassigned && <button type="button" className="secondaryButton" onClick={() => assignIdentityDocument(doc.identity_document_id)}>Take case</button>}
                {canReview && <button type="button" onClick={() => verifyIdentityDocument(doc.identity_document_id)}>Verify identity</button>}
                {canReview && <button type="button" className="dangerButton subtleDangerButton" onClick={() => rejectIdentityDocument(doc.identity_document_id)}>Reject</button>}
              </div>
            </article>
            );
          })}
          {showingSkills && pendingSkills.length === 0 && <AdminEmptyState title="No pending skills" text="New worker skills will appear here." />}
          {showingSkills && pendingSkills.map((skill) => (
            <article className="adminQueueRow" key={skill.worker_skill_id}>
              <div>
                <strong>{categoryTitle(skill.category_name)}</strong>
                <span>{skill.worker_full_name} - {skill.worker_user_email}</span>
              </div>
              <div className="adminQueueMeta">
                <span className="statusPill pending">{skill.experience_level}</span>
                <span>{skill.price_base} KZT</span>
                <span>Skill #{skill.worker_skill_id}</span>
              </div>
              <EvidenceLinks value={skill.evidence_files} />
              <button type="button" onClick={() => verifySkill(skill.worker_skill_id)}>Verify skill</button>
            </article>
          ))}
          {showingUpgrades && pendingUpgrades.length === 0 && <AdminEmptyState title="No upgrade requests" text="Requests for junior to middle or senior will appear here." />}
          {showingUpgrades && pendingUpgrades.map((request) => (
            <article className="adminQueueRow" key={request.upgrade_request_id}>
              <div>
                <strong>{categoryTitle(request.category_name)}</strong>
                <span>{request.worker_full_name} - {request.worker_user_email}</span>
              </div>
              <div className="adminQueueMeta">
                <span>{request.current_level} to {request.requested_level}</span>
                <span>Skill #{request.worker_skill_id}</span>
                {request.created_at && <span>{formatDateTime(request.created_at)}</span>}
              </div>
              {request.admin_note && <small>{request.admin_note}</small>}
              <EvidenceLinks value={request.evidence_files} />
              <button type="button" onClick={() => verifySkillUpgrade(request.upgrade_request_id)}>Approve upgrade</button>
            </article>
          ))}
        </div>
      </section>
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
        <button
          key={file}
          type="button"
          onClick={() => openFilePreview({ url: apiURL(file), name: file.split("/").pop() || "Evidence" })}
        >
          Open evidence
        </button>
      ))}
    </div>
  );
}

function BookingsPanel({ token, canProgress, canConfirm, onProgress, onNavigate, showRequests = false }) {
  const [items, setItems] = useState([]);
  const [requests, setRequests] = useState([]);
  const [requestHistoryOpen, setRequestHistoryOpen] = useState(false);
  const [error, setError] = useState("");
  const [requestError, setRequestError] = useState("");
  const [message, setMessage] = useState("");

  const load = useCallback(() => {
    apiGet("/api/bookings/my", token).then((data) => setItems(Array.isArray(data) ? data : data.bookings || [])).catch((err) => setError(err.message));
  }, [token]);

  useEffect(() => load(), [load]);

  const loadRequests = useCallback(() => {
    if (!showRequests) {
      return;
    }
    apiGet("/api/requests/my", token)
      .then((data) => {
        setRequestError("");
        setRequests(Array.isArray(data) ? data : data.requests || []);
      })
      .catch((err) => setRequestError(err.message));
  }, [showRequests, token]);

  useEffect(() => loadRequests(), [loadRequests]);

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
                <button className="secondaryButton" onClick={() => {
                  localStorage.setItem("workers_marketplace_report_booking", String(item.booking_id || item.id));
                  onNavigate?.("reports");
                }}>Report</button>
                {String(item.status).toLowerCase() === "scheduled" && <button onClick={() => onProgress(item.booking_id || item.id, "start")}>Start route</button>}
                {String(item.status).toLowerCase() === "in_progress" && <button className="secondaryButton" onClick={() => onProgress(item.booking_id || item.id, "complete")}>Send proof</button>}
              </div>
            )}
            {canConfirm && (canChatBooking(item) || String(item.status).toLowerCase() === "awaiting_confirmation") && (
              <div className="rowActions bookingActions">
                {canChatBooking(item) && (
                  <>
                    <button className="secondaryButton" onClick={() => openChat(item.booking_id || item.id)}>Chat</button>
                    <button className="secondaryButton" onClick={() => {
                      localStorage.setItem("workers_marketplace_report_booking", String(item.booking_id || item.id));
                      onNavigate?.("reports");
                    }}>Report</button>
                  </>
                )}
                {String(item.status).toLowerCase() === "awaiting_confirmation" && (
                  <button className="primaryBookingAction" onClick={() => confirmCompletion(item.booking_id || item.id)}>Confirm completion</button>
                )}
              </div>
            )}
            {canConfirm && String(item.status).toLowerCase() === "completed" && (
              <BookingReviewBlock item={item} token={token} onSaved={load} />
            )}
          </article>
        ))}
      </div>
      {showRequests && (
        <section className="secondaryPanel requestArchive">
          <div className="sectionTitleRow">
            <div>
              <h3>Request history</h3>
              <span>Created requests are kept here quietly instead of cluttering the main menu.</span>
            </div>
            <div className="rowActions">
              <button className="secondaryButton" type="button" onClick={() => setRequestHistoryOpen((value) => !value)}>
                {requestHistoryOpen ? "Hide" : `Show ${requests.length ? `(${requests.length})` : ""}`} history
              </button>
              <button className="secondaryButton" type="button" onClick={loadRequests}>Refresh</button>
            </div>
          </div>
          {requestHistoryOpen && (
            <>
              <Messages error={requestError} />
              <div className="requestGrid compact">
                {requests.length === 0 && <EmptyState title="No service requests yet" text="Requests appear here after you choose a worker." />}
                {requests.map((item) => (
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
            </>
          )}
        </section>
      )}
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
        {item.review_photo_url && (
          <button
            className="reviewPhotoButton"
            type="button"
            onClick={() => openFilePreview({ url: apiURL(item.review_photo_url), name: "Review photo", type: "image" })}
          >
            <img className="reviewPhoto" src={apiURL(item.review_photo_url)} alt="" />
          </button>
        )}
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
  const [chatFolder, setChatFolder] = useState("active");
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [selfAvatarURL, setSelfAvatarURL] = useState("");
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
  const activeChatArchived = isArchivedBookingStatus(activeChat?.status) || isArchivedBookingStatus(activeChat?.booking_status || activeBookingStatus);
  const activeBookingPrice = parseMoney(activeBooking?.final_price || 0);
  const partnerAvatarURL = activeBooking?.counterparty_photo_url ? apiURL(activeBooking.counterparty_photo_url) : "";
  const messageListRef = useRef(null);
  const activeChats = useMemo(() => chats.filter((chat) => !isArchivedBookingStatus(chat.status) && !isArchivedBookingStatus(chat.booking_status)), [chats]);
  const archivedChats = useMemo(() => chats.filter((chat) => isArchivedBookingStatus(chat.status) || isArchivedBookingStatus(chat.booking_status)), [chats]);
  const visibleChats = useMemo(() => (chatFolder === "archived" ? archivedChats : activeChats), [activeChats, archivedChats, chatFolder]);

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
    const endpoint = role === "worker" ? "/api/worker/profile" : role === "customer" ? "/api/customer/profile" : "";
    if (!endpoint) {
      setSelfAvatarURL("");
      return;
    }
    apiGet(endpoint, token)
      .then((data) => setSelfAvatarURL(data?.profile_photo_url ? apiURL(data.profile_photo_url) : ""))
      .catch(() => setSelfAvatarURL(""));
  }, [role, token]);

  useEffect(() => {
    if (visibleChats.length === 0) {
      setActiveChatID("");
      return;
    }
    if (!visibleChats.some((chat) => String(chat.chat_id) === String(activeChatID))) {
      setActiveChatID(String(visibleChats[0].chat_id));
    }
  }, [activeChatID, visibleChats]);

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
    const messageList = messageListRef.current;
    if (!messageList) return;
    messageList.scrollTo({
      top: messageList.scrollHeight,
      behavior: "smooth",
    });
  }, [messages]);

  const postChatText = async (text) => {
    if (!activeChatID) return null;
    if (activeChatArchived) {
      throw new Error("This chat is archived.");
    }
    const sent = await apiPost(`/api/chats/${activeChatID}/messages`, token, { content: text });
    setMessages((current) => [...current.filter((msg) => msg.message_id !== sent.message_id), sent]);
    return sent;
  };

  const send = async (event) => {
    event.preventDefault();
    if (!activeChatID) return;
    if (!content.trim() && !attachment) return;
    if (activeChatArchived) {
      setError("This chat is archived. You can read it, but cannot send new messages.");
      return;
    }
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

  const canWorkerSetPrice = !activeChatArchived && role === "worker" && (activeBookingStatus === "price_pending" || activeBookingStatus === "scheduled");
  const canCustomerDecidePrice = !activeChatArchived && role === "customer" && activeBookingStatus === "price_pending";

  return (
    <section className="pagePanel chatPage">
      <SectionHeader title="Chat" text="Talk about details, timing, price and files." />
      <Messages message={message} error={error} />
      <div className="chatLayout">
        <aside className="chatList">
          <div className="chatFolderTabs" role="tablist" aria-label="Chat folders">
            <button type="button" className={chatFolder === "active" ? "active" : ""} onClick={() => setChatFolder("active")}>
              Active <span>{activeChats.length}</span>
            </button>
            <button type="button" className={chatFolder === "archived" ? "active" : ""} onClick={() => setChatFolder("archived")}>
              Completed <span>{archivedChats.length}</span>
            </button>
          </div>
          {visibleChats.length === 0 && (
            <EmptyState
              title={chatFolder === "active" ? "No active chats" : "No completed chats"}
              text={chatFolder === "active" ? "Open a chat from a booking card." : "Completed booking chats will stay here as history."}
            />
          )}
          {visibleChats.map((chat) => (
            <button
              key={chat.chat_id}
              className={String(activeChatID) === String(chat.chat_id) ? "active" : ""}
              type="button"
              onClick={() => setActiveChatID(String(chat.chat_id))}
            >
              <strong>Booking #{chat.booking_id}</strong>
              <span>{chat.unread_count ? `${chat.unread_count} unread` : chat.booking_status || chat.status}</span>
            </button>
          ))}
        </aside>
        <div className="chatConversation">
          {activeChatArchived && (
            <div className="chatArchiveNotice">
              This booking is completed. The chat is kept as history and is read-only.
            </div>
          )}
          {canWorkerSetPrice && (
            <form className="chatPriceBar" onSubmit={setBookingPrice}>
              <Field label="Booking price, KZT" light>
                <input value={priceDraft} onChange={(event) => setPriceDraft(event.target.value)} inputMode="numeric" placeholder="Enter amount" />
              </Field>
              <button disabled={!activeChatID}>Set price</button>
            </form>
          )}
          {canCustomerDecidePrice && (
            <div className="chatPriceDecision">
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
          <div className="chatMessageList" ref={messageListRef}>
            {!activeChatID && <EmptyState title="Choose chat" text="Select a booking chat on the left." />}
            {messages.map((msg) => (
              <ChatBubble
                key={msg.message_id}
                msg={msg}
                own={Number(msg.sender_user_id) === Number(currentUserID)}
                ownAvatarURL={selfAvatarURL}
                partnerAvatarURL={partnerAvatarURL}
              />
            ))}
          </div>
          <form className={activeChatArchived ? "chatComposer archived" : "chatComposer"} onSubmit={send}>
            <textarea value={content} onChange={(event) => setContent(event.target.value)} onKeyDown={submitTextareaOnEnter} placeholder={activeChatArchived ? "This chat is archived" : "Write a message..."} disabled={activeChatArchived} />
            <label className="fileButton chatFileButton" aria-label="Attach file" title="Attach file">
              <span aria-hidden="true">+</span>
              <span className="visuallyHidden">Attach file</span>
              <input type="file" accept="image/png,image/jpeg,image/webp,video/mp4,video/quicktime,video/webm,application/pdf,.doc,.docx,.txt" disabled={activeChatArchived} onChange={(event) => setAttachment(event.target.files?.[0] || null)} />
            </label>
            <button className="chatSendButton" aria-label="Send message" title="Send message" disabled={activeChatArchived || !activeChatID || (!content.trim() && !attachment)}>
              <span aria-hidden="true" />
              <span className="visuallyHidden">Send</span>
            </button>
            {attachment && <span className="muted">{attachment.name}</span>}
          </form>
        </div>
      </div>
    </section>
  );
}

function ChatBubble({ msg, own, ownAvatarURL, partnerAvatarURL }) {
  const label = own ? "You" : "Partner";
  const avatarURL = msg.sender_avatar_url ? apiURL(msg.sender_avatar_url) : own ? ownAvatarURL : partnerAvatarURL;
  return (
    <article className={own ? "chatBubble own" : "chatBubble"}>
      <div className="chatAvatar" aria-hidden="true">
        {avatarURL ? <img src={avatarURL} alt="" /> : own ? "Y" : "P"}
      </div>
      <div className="chatBubbleBody">
        <small>{label}</small>
        {msg.content && !(msg.attachment_url && msg.content === "Attachment") && <span>{msg.content}</span>}
        {msg.attachment_url && (
          <AttachmentPreview msg={msg} />
        )}
        <footer className="chatMeta">
          <time dateTime={msg.created_at || msg.createdAt || ""}>{formatChatTime(msg.created_at || msg.createdAt)}</time>
          {own && <span className={msg.read_at || msg.readAt ? "read" : ""}>{msg.read_at || msg.readAt ? "✓✓" : "✓"}</span>}
        </footer>
      </div>
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
      <button
        className="chatAttachment media"
        type="button"
        onClick={() => openFilePreview({ url, name, type: msg.attachment_type || "image" })}
      >
        <img src={url} alt={name} />
      </button>
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
    <button
      className="chatAttachment file"
      type="button"
      onClick={() => openFilePreview({ url, name, type: msg.attachment_type || "" })}
    >
      {name}
    </button>
  );
}

function ReportsPanel({ token, role, staff = false, onOpenProfile }) {
  const [reports, setReports] = useState([]);
  const [reportBookings, setReportBookings] = useState([]);
  const [activeID, setActiveID] = useState(() => localStorage.getItem("workers_marketplace_active_report") || "");
  const [activeSide, setActiveSide] = useState(() => localStorage.getItem("workers_marketplace_active_report_side") || "reporter");
  const [reportPerspective, setReportPerspective] = useState("created");
  const [showCreateForm, setShowCreateForm] = useState(() => Boolean(localStorage.getItem("workers_marketplace_report_booking")));
  const [supportChatOpen, setSupportChatOpen] = useState(false);
  const [messages, setMessages] = useState([]);
  const [form, setForm] = useState(() => ({
    booking_id: localStorage.getItem("workers_marketplace_report_booking") || "",
    reason: "bad_quality",
    description: "",
  }));
  const [files, setFiles] = useState([]);
  const [messageText, setMessageText] = useState("");
  const [attachment, setAttachment] = useState(null);
  const [penalty, setPenalty] = useState({ penalty_type: "warning", days: "7", reason: "" });
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  const loadReports = useCallback(() => {
    apiGet("/api/reports", token)
      .then((data) => {
        const next = Array.isArray(data) ? data : data.reports || [];
        setReports(next);
        setActiveID((current) => {
          const storedID = localStorage.getItem("workers_marketplace_active_report");
          if (storedID && next.some((report) => String(report.report_id) === storedID)) {
            return storedID;
          }
          return current || String(next[0]?.report_id || "");
        });
      })
      .catch((err) => setError(err.message));
  }, [token]);

  const loadReportBookings = useCallback(() => {
    if (staff) return;
    apiGet("/api/bookings/my", token)
      .then((data) => {
        const next = Array.isArray(data) ? data : data.bookings || [];
        setReportBookings(next);
        setForm((current) => {
          if (current.booking_id || next.length === 0) return current;
          return { ...current, booking_id: String(next[0].booking_id || next[0].id || "") };
        });
      })
      .catch((err) => setError(err.message));
  }, [staff, token]);

  const loadMessages = useCallback((reportID, side) => {
    if (!reportID) {
      setMessages([]);
      return;
    }
    apiGet(`/api/reports/${reportID}/messages?side=${encodeURIComponent(side || "reporter")}`, token)
      .then((data) => setMessages(Array.isArray(data) ? data : data.messages || []))
      .catch((err) => setError(err.message));
  }, [token]);

  useEffect(() => {
    loadReports();
    const intervalID = window.setInterval(loadReports, 5000);
    return () => window.clearInterval(intervalID);
  }, [loadReports]);

  useEffect(() => {
    loadReportBookings();
  }, [loadReportBookings]);

  useEffect(() => {
    loadMessages(activeID, activeSide);
    if (!activeID) return undefined;
    localStorage.setItem("workers_marketplace_active_report", String(activeID));
    localStorage.setItem("workers_marketplace_active_report_side", String(activeSide));
    const intervalID = window.setInterval(() => loadMessages(activeID, activeSide), 4000);
    return () => window.clearInterval(intervalID);
  }, [activeID, activeSide, loadMessages]);

  const currentUserID = tokenUserID(token);
  const createdReports = reports.filter((item) => Number(item.reporter_user_id) === Number(currentUserID));
  const againstReports = reports.filter((item) => Number(item.reported_user_id) === Number(currentUserID));
  const staffVisibleReports = staff && role === "manager"
    ? reports.filter((item) => !item.assigned_manager_id || Number(item.assigned_manager_id) === Number(currentUserID))
    : reports;
  const visibleReports = staff ? staffVisibleReports : reportPerspective === "against" ? againstReports : createdReports;
  const activeReport = (staff ? reports : visibleReports).find((item) => String(item.report_id) === String(activeID));
  const activeUserSide = activeReport && Number(activeReport.reported_user_id) === Number(currentUserID) ? "reported" : "reporter";
  const selectedBooking = reportBookings.find((item) => String(item.booking_id || item.id) === String(form.booking_id));
  const fileSummary = files.length === 0 ? "No files selected" : files.map((file) => file.name).join(", ");
  const reportListTitle = staff ? "Report queue" : "My cases";
  const reporterLabel = (report) => report?.reporter_email || report?.reporter_name || "Reporter";
  const reportedLabel = (report) => report?.reported_email || report?.reported_name || "Reported";
  const activeReportClosed = activeReport ? !isOpenReportStatus(activeReport.status) : false;

  useEffect(() => {
    if (staff || !activeReport) return;
    if (activeSide !== activeUserSide) {
      setActiveSide(activeUserSide);
    }
  }, [activeReport, activeSide, activeUserSide, staff]);

  useEffect(() => {
    if (visibleReports.length === 0) {
      if (!staff) setActiveID("");
      return;
    }
    if (!visibleReports.some((report) => String(report.report_id) === String(activeID))) {
      setActiveID(String(visibleReports[0].report_id));
    }
  }, [activeID, staff, visibleReports]);

  const createReport = async (event) => {
    event.preventDefault();
    setError("");
    setMessage("");
    try {
      const body = new FormData();
      if (form.booking_id) body.append("booking_id", form.booking_id);
      body.append("reason", form.reason);
      body.append("description", form.description);
      files.forEach((file) => body.append("report_files", file));
      const created = await apiMultipart("/api/reports", token, body);
      localStorage.removeItem("workers_marketplace_report_booking");
      setForm({ booking_id: "", reason: "bad_quality", description: "" });
      setFiles([]);
      setShowCreateForm(false);
      setSupportChatOpen(true);
      setMessage("Report created. You can continue in support chat.");
      setActiveID(String(created.report_id || ""));
      loadReports();
      loadReportBookings();
    } catch (err) {
      setError(err.message);
    }
  };

  const sendReportMessage = async (event) => {
    event.preventDefault();
    if (!activeID) return;
    if (!messageText.trim() && !attachment) return;
    if (activeReportClosed) {
      setError("This support case is closed. You can read it, but cannot send new messages.");
      return;
    }
    setError("");
    try {
      let sent;
      if (attachment) {
        const body = new FormData();
        body.append("message_text", messageText);
        body.append("attachment", attachment);
        sent = await apiMultipart(`/api/reports/${activeID}/messages?side=${encodeURIComponent(activeSide)}`, token, body);
      } else {
        sent = await apiPost(`/api/reports/${activeID}/messages?side=${encodeURIComponent(activeSide)}`, token, { message_text: messageText });
      }
      setMessages((current) => [...current, sent]);
      setMessageText("");
      setAttachment(null);
      loadReports();
    } catch (err) {
      setError(err.message);
    }
  };

  const applyPenalty = async () => {
    if (!activeReport) return;
    setError("");
    setMessage("");
    try {
      await apiPost(`/api/admin/reports/${activeReport.report_id}/penalty`, token, {
        target_user_id: activeReport.reported_user_id,
        penalty_type: penalty.penalty_type,
        reason: penalty.reason || activeReport.reason,
        days: Number(penalty.days || 0),
      });
      setMessage("Penalty applied and report resolved.");
      loadReports();
    } catch (err) {
      setError(err.message);
    }
  };

  const closeReport = async (status) => {
    if (!activeReport) return;
    setError("");
    setMessage("");
    try {
      await apiPatch(`/api/admin/reports/${activeReport.report_id}/close`, token, {
        status,
        resolution: penalty.reason || "Closed by support",
      });
      setMessage("Report closed.");
      loadReports();
    } catch (err) {
      setError(err.message);
    }
  };

  const assignReport = async () => {
    if (!activeReport) return;
    setError("");
    setMessage("");
    try {
      const assigned = await apiPatch(`/api/admin/reports/${activeReport.report_id}/assign`, token, {});
      setReports((current) => current.map((item) => (
        Number(item.report_id) === Number(assigned.report_id) ? assigned : item
      )));
      setMessage("Report assigned to you.");
      loadReports();
      window.dispatchEvent(new CustomEvent("wm-notifications-updated"));
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="pagePanel reportsPage">
      <SectionHeader title={staff ? "Reports" : "My reports"} text={staff ? "Review disputes and apply penalties when needed." : "Create a support case for a booking issue."} />
      <Messages message={message} error={error} />
      <div className={staff ? "reportsLayout staffReportsLayout" : "reportsLayout"}>
        <div className="reportSidebar">
          {!staff && (
            <div className="reportCreateLauncher">
              <button type="button" onClick={() => setShowCreateForm((current) => !current)}>
                {showCreateForm ? "Close form" : "+ New case"}
              </button>
              <span>{showCreateForm ? "Fill the case details below." : "Need help? Start a case, then chat with support."}</span>
            </div>
          )}
          {!staff && (
            <div className="reportPerspectiveTabs" role="tablist" aria-label="Report folders">
              <button
                type="button"
                className={reportPerspective === "created" ? "active" : ""}
                onClick={() => setReportPerspective("created")}
              >
                My reports
                <span>{createdReports.length}</span>
              </button>
              <button
                type="button"
                className={reportPerspective === "against" ? "active" : ""}
                onClick={() => setReportPerspective("against")}
              >
                Against me
                <span>{againstReports.length}</span>
              </button>
            </div>
          )}
          {!staff && showCreateForm && (
            <form className="toolCard reportCreateForm" onSubmit={createReport}>
              <div className="reportFormHeader">
                <div>
                  <h3>New support case</h3>
                  <p>Tell support what happened. Proof is optional.</p>
                </div>
                <span className="reportHintBadge">Support</span>
              </div>
              <div className="reportFormGrid">
                <Field label="Booking" light>
                  <select value={form.booking_id} onChange={(event) => setForm({ ...form, booking_id: event.target.value })} required>
                    <option value="">Choose booking</option>
                    {reportBookings.map((booking) => {
                      const id = booking.booking_id || booking.id;
                      const title = booking.address || booking.description || "Booking";
                      return <option key={id} value={id}>Booking #{id} - {title}</option>;
                    })}
                  </select>
                </Field>
                <Field label="Reason" light>
                  <select value={form.reason} onChange={(event) => setForm({ ...form, reason: event.target.value })}>
                    <option value="bad_quality">Bad quality</option>
                    <option value="no_show">Worker/customer did not arrive</option>
                    <option value="rude_behavior">Rude behavior</option>
                    <option value="fake_evidence">Fake evidence</option>
                    <option value="payment_disagreement">Payment disagreement</option>
                    <option value="other">Other</option>
                  </select>
                </Field>
              </div>
              {selectedBooking && (
                <div className="selectedReportBooking">
                  <div>
                    <span>Selected booking</span>
                    <strong>Booking #{selectedBooking.booking_id || selectedBooking.id}</strong>
                  </div>
                  <p>{selectedBooking.address || selectedBooking.description || "No address"}</p>
                  <div>
                    <small>{selectedBooking.final_price ? `${formatMoney(parseMoney(selectedBooking.final_price))} KZT` : "Price was set in chat"}</small>
                    <span className={`statusPill ${selectedBooking.status || ""}`}>{selectedBooking.status || "booking"}</span>
                  </div>
                </div>
              )}
              <Field label="Description" light><textarea value={form.description} onChange={(event) => setForm({ ...form, description: event.target.value })} placeholder="Explain what happened..." required /></Field>
              <div className="reportUploadRow">
                <label className="fileButton reportFileButton" aria-label="Attach proof files" title="Attach proof files">
                  <span aria-hidden="true">+</span>
                  <strong>Attach proof</strong>
                  <input type="file" multiple onChange={(event) => setFiles(Array.from(event.target.files || []))} />
                </label>
                <span title={fileSummary}>{fileSummary}</span>
              </div>
              <button className="reportCreateButton" disabled={!form.booking_id}>Create report</button>
            </form>
          )}
          <aside className="reportList" aria-label={reportListTitle}>
            <div className="reportListHeader">
              <strong>{reportListTitle}</strong>
              <span>{visibleReports.length}</span>
            </div>
            {visibleReports.length === 0 && <EmptyState title="No reports" text="Reports will appear here." />}
            {visibleReports.map((report) => (
              <button key={report.report_id} className={String(activeID) === String(report.report_id) ? "active" : ""} onClick={() => {
                setActiveID(String(report.report_id));
                setActiveSide(staff ? activeSide : Number(report.reported_user_id) === Number(currentUserID) ? "reported" : "reporter");
              }}>
                <div className="reportCardTop">
                  <strong>Report #{report.report_id}</strong>
                  <span className={`statusPill ${report.status || ""}`}>{reportStatusLabel(report.status)}</span>
                </div>
                <span>{reportReasonLabel(report.reason)}</span>
                <small>{staff ? `${reporterLabel(report)} -> ${reportedLabel(report)}` : `Booking #${report.booking_id || "-"}`}</small>
              </button>
            ))}
          </aside>
        </div>
        <section className="reportDetail">
          {!activeReport && <EmptyState title={visibleReports.length === 0 ? "No reports yet" : "Choose report"} text={visibleReports.length === 0 ? "Create a support case and it will appear here." : "Select a report from the list."} />}
          {activeReport && (
            <>
              <div className="toolCard reportSummary">
                <div className="reportSummaryTop">
                  <div>
                    <h3>Report #{activeReport.report_id}</h3>
                    <small>Booking #{activeReport.booking_id || "-"} | {reporterLabel(activeReport)} {"->"} {reportedLabel(activeReport)}</small>
                  </div>
                  <div className="reportSummaryActions">
                    <span className={`statusPill ${activeReport.status || ""}`}>{reportStatusLabel(activeReport.status)}</span>
                    {staff && !activeReport.assigned_manager_id && (
                      <button className="secondaryButton" type="button" onClick={assignReport}>Take case</button>
                    )}
                    {staff && activeReport.assigned_manager_id && (
                      <span className="statusPill in_review">
                        {Number(activeReport.assigned_manager_id) === Number(currentUserID) ? "Assigned to me" : `Assigned #${activeReport.assigned_manager_id}`}
                      </span>
                    )}
                    {staff ? (
                      <div className="reportConversationButtons">
                        <button
                          className={activeSide === "reporter" ? "reportChatOpenButton active" : "reportChatOpenButton"}
                          type="button"
                          onClick={() => {
                            setActiveSide("reporter");
                            setSupportChatOpen(true);
                          }}
                          aria-label="Open reporter chat"
                          title="Open reporter chat"
                        >
                          <span aria-hidden="true" />
                          Reporter
                        </button>
                        <button
                          className={activeSide === "reported" ? "reportChatOpenButton active" : "reportChatOpenButton"}
                          type="button"
                          onClick={() => {
                            setActiveSide("reported");
                            setSupportChatOpen(true);
                          }}
                          aria-label="Open reported user chat"
                          title="Open reported user chat"
                        >
                          <span aria-hidden="true" />
                          Reported
                        </button>
                      </div>
                    ) : (
                      <button className="reportChatOpenButton" type="button" onClick={() => setSupportChatOpen(true)} aria-label="Open support chat" title="Open support chat">
                        <span aria-hidden="true" />
                        Support chat
                      </button>
                    )}
                  </div>
                </div>
                {staff && (
                  <div className="reportParticipantLinks">
                    <button type="button" className="secondaryButton" onClick={() => onOpenProfile?.(activeReport.reporter_user_id)}>
                      Reporter: {reporterLabel(activeReport)}
                    </button>
                    <button type="button" className="secondaryButton" onClick={() => onOpenProfile?.(activeReport.reported_user_id)}>
                      Reported: {reportedLabel(activeReport)}
                    </button>
                  </div>
                )}
                <div className="reportInfoGrid">
                  <span>
                    <small>Reason</small>
                    <strong>{reportReasonLabel(activeReport.reason)}</strong>
                  </span>
                  <span>
                    <small>Booking</small>
                    <strong>#{activeReport.booking_id || "-"}</strong>
                  </span>
                </div>
                <p>{activeReport.description || "No description"}</p>
              </div>
              {staff && (
                <div className="toolCard reportPenaltyBox">
                  <h3>Support decision</h3>
                  <div className="toolbarGrid">
                    <Field label="Penalty" light>
                      <select value={penalty.penalty_type} onChange={(event) => setPenalty({ ...penalty, penalty_type: event.target.value })}>
                        <option value="warning">Warning</option>
                        <option value="temporary_suspend">Temporary suspend</option>
                        <option value="unverify_skills">Unverify worker skills</option>
                        {role === "admin" && <option value="block_user">Block user</option>}
                      </select>
                    </Field>
                    <Field label="Days for suspend" light><input value={penalty.days} onChange={(event) => setPenalty({ ...penalty, days: event.target.value })} /></Field>
                  </div>
                  <Field label="Resolution note" light><textarea value={penalty.reason} onChange={(event) => setPenalty({ ...penalty, reason: event.target.value })} placeholder="What decision was made?" /></Field>
                  <div className="rowActions">
                    <button type="button" onClick={applyPenalty}>Apply penalty</button>
                    <button type="button" className="secondaryButton" onClick={() => closeReport("rejected")}>Reject report</button>
                    <button type="button" className="secondaryButton" onClick={() => closeReport("resolved")}>Close without penalty</button>
                  </div>
                </div>
              )}
            </>
          )}
        </section>
      </div>
      {activeReport && supportChatOpen && (
        <div className="reportChatPanel" role="dialog" aria-label="Support chat" aria-modal="false">
          <div className="reportChatHeader">
            <div>
              <strong>Support chat</strong>
              <span>
                Report #{activeReport.report_id} · {staff
                  ? activeSide === "reported" ? `With ${reportedLabel(activeReport)}` : `With ${reporterLabel(activeReport)}`
                  : activeSide === "reported" ? "You were reported" : "Your report"}
              </span>
            </div>
            <button className="reportChatCloseButton" type="button" onClick={() => setSupportChatOpen(false)} aria-label="Close support chat" title="Close support chat">×</button>
          </div>
          <div className="reportMessageList">
            {messages.length === 0 && <EmptyState title="No dispute messages" text="Write a message to start the support chat." />}
            {messages.map((msg) => (
              <ReportChatBubble
                key={msg.message_id}
                msg={msg}
                own={Number(msg.sender_user_id) === tokenUserID(token)}
                staff={Number(msg.sender_user_id) !== Number(activeReport.reporter_user_id) && Number(msg.sender_user_id) !== Number(activeReport.reported_user_id)}
                onOpenProfile={staff ? onOpenProfile : undefined}
              />
            ))}
          </div>
          {activeReportClosed ? (
            <div className="chatArchiveNotice">
              This support case is closed. The chat is kept as history and is read-only.
            </div>
          ) : (
            <form className="reportMessageForm" onSubmit={sendReportMessage}>
              <textarea value={messageText} onChange={(event) => setMessageText(event.target.value)} onKeyDown={submitTextareaOnEnter} placeholder="Write to support..." />
              <label className="fileButton chatFileButton reportMessageFileButton" aria-label="Attach file" title="Attach file">
                <span aria-hidden="true">+</span>
                <span className="visuallyHidden">Attach file</span>
                <input type="file" accept="image/png,image/jpeg,image/webp,video/mp4,video/quicktime,video/webm,application/pdf,.doc,.docx,.txt" onChange={(event) => setAttachment(event.target.files?.[0] || null)} />
              </label>
              <button className="chatSendButton" aria-label="Send message" title="Send message" disabled={!messageText.trim() && !attachment}>
                <span aria-hidden="true" />
                <span className="visuallyHidden">Send</span>
              </button>
              {attachment && <span className="muted">{attachment.name}</span>}
            </form>
          )}
        </div>
      )}
    </section>
  );
}

function ReportChatBubble({ msg, own, staff, onOpenProfile }) {
  const label = staff ? "Support" : own ? "You" : msg.sender_name || "User";
  const initials = staff ? "M" : own ? "Y" : initialsOf(msg.sender_name || "U");
  const avatarURL = staff ? STAFF_AVATAR_URL : msg.sender_avatar_url ? apiURL(msg.sender_avatar_url) : "";
  const canOpenProfile = Boolean(onOpenProfile && !staff && ["customer", "worker"].includes(String(msg.sender_role || "")));
  const avatar = (
    <>
      {avatarURL ? <img src={avatarURL} alt="" /> : initials}
    </>
  );
  return (
    <article className={`${own ? "chatBubble own" : "chatBubble"} ${staff ? "supportBubble" : ""}`}>
      {canOpenProfile ? (
        <button
          className={`chatAvatar chatAvatarButton ${staff ? "staffAvatar" : ""}`}
          type="button"
          onClick={() => onOpenProfile(msg.sender_user_id)}
          aria-label={`Open ${msg.sender_email || msg.sender_name || "user"} profile`}
          title={`Open ${msg.sender_email || msg.sender_name || "user"} profile`}
        >
          {avatar}
        </button>
      ) : (
        <div className={`chatAvatar ${staff ? "staffAvatar" : ""}`} aria-hidden="true">
          {avatar}
        </div>
      )}
      <div className="chatBubbleBody">
        <small>{label}</small>
        {msg.message_text && <span>{msg.message_text}</span>}
        {msg.attachment_url && <AttachmentPreview msg={{
          attachment_url: msg.attachment_url,
          attachment_name: msg.attachment_name || "Attachment",
          attachment_type: msg.attachment_type || "",
        }} />}
        <footer className="chatMeta">
          <time dateTime={msg.created_at || ""}>{formatChatTime(msg.created_at)}</time>
          {own && <span className={msg.read_at ? "read" : ""}>{msg.read_at ? "✓✓" : "✓"}</span>}
        </footer>
      </div>
    </article>
  );
}

function reportReasonLabel(reason) {
  return reportReasonLabels[String(reason || "").toLowerCase()] || reason || "Other";
}

function reportStatusLabel(status) {
  return reportStatusLabels[String(status || "").toLowerCase()] || status || "Open";
}

function isOpenReportStatus(status) {
  return !["closed", "rejected", "resolved"].includes(String(status || "open").toLowerCase());
}

function isArchivedBookingStatus(status) {
  return ["completed", "cancelled", "closed"].includes(String(status || "").toLowerCase());
}

function initialsOf(name) {
  return String(name || "")
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part[0])
    .join("")
    .toUpperCase() || "U";
}

function FilePreviewPortal() {
  const [file, setFile] = useState(null);

  useEffect(() => {
    const open = (event) => setFile(event.detail || null);
    const close = (event) => {
      if (event.key === "Escape") setFile(null);
    };
    window.addEventListener("wm-file-preview", open);
    window.addEventListener("keydown", close);
    return () => {
      window.removeEventListener("wm-file-preview", open);
      window.removeEventListener("keydown", close);
    };
  }, []);

  if (!file?.url) {
    return null;
  }

  const url = file.url;
  const name = file.name || "File preview";
  const type = String(file.type || "").toLowerCase();
  const lowerURL = String(url).toLowerCase();
  const isImage = type.startsWith("image") || /\.(jpg|jpeg|png|webp)$/i.test(lowerURL);
  const isVideo = type.startsWith("video") || /\.(mp4|webm|mov)$/i.test(lowerURL);
  const isPdf = type.includes("pdf") || /\.pdf$/i.test(lowerURL);

  return (
    <div className="filePreviewOverlay" role="dialog" aria-modal="true" aria-label={name} onMouseDown={() => setFile(null)}>
      <div className="filePreviewModal" onMouseDown={(event) => event.stopPropagation()}>
        <header>
          <strong>{name}</strong>
          <button type="button" aria-label="Close preview" onClick={() => setFile(null)}>×</button>
        </header>
        <div className="filePreviewBody">
          {isImage && <img src={url} alt={name} />}
          {isVideo && <video controls autoPlay src={url} />}
          {isPdf && <iframe title={name} src={url} />}
          {!isImage && !isVideo && !isPdf && (
            <div className="filePreviewFallback">
              <p>This file type cannot be previewed inline.</p>
              <a href={url} target="_blank" rel="noreferrer">Open file</a>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function NotificationsPanel({ token, onNavigate }) {
  const [items, setItems] = useState([]);
  const [mode, setMode] = useState("unread");
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
      } else if (item.action_type === "booking_map") {
        onNavigate?.("find");
      } else if (item.action_type === "report" && item.action_ref) {
        localStorage.setItem("workers_marketplace_active_report", String(item.action_ref));
        onNavigate?.("reports");
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

  const unreadItems = items.filter((item) => !item.is_read);
  const readItems = items.filter((item) => item.is_read);
  const visibleItems = mode === "unread" ? unreadItems : readItems;

  return (
    <section className="pagePanel">
      <SectionHeader title="Notifications" text="System messages and booking updates." />
      <div className="notificationTabs">
        <button className={mode === "unread" ? "active" : ""} type="button" onClick={() => setMode("unread")}>
          Unread <span>{unreadItems.length}</span>
        </button>
        <button className={mode === "read" ? "active" : ""} type="button" onClick={() => setMode("read")}>
          Read <span>{readItems.length}</span>
        </button>
        <button className="secondaryButton fitButton" onClick={markAll} disabled={unreadItems.length === 0}>Mark all read</button>
      </div>
      <Messages error={error} />
      <div className="dataList">
        {visibleItems.length === 0 && <EmptyState title={mode === "unread" ? "No unread notifications" : "No read notifications"} text={mode === "unread" ? "Fresh alerts will appear here." : "Read alerts stay here for one week."} />}
        {visibleItems.map((item) => (
          <article className={`dataRow notificationRow ${item.is_read ? "read" : "unread"}`} key={item.notification_id || item.id}>
            <strong>{item.title || item.type || "Notification"}</strong>
            <span>{item.message || item.body || ""}</span>
            {item.created_at && <small>{formatDateTime(item.created_at)}</small>}
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
  const [avatarDraft, setAvatarDraft] = useState(null);
  const [avatarEditorOpen, setAvatarEditorOpen] = useState(false);
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

  useEffect(() => () => revokeAvatarDraft(avatarDraft), [avatarDraft?.previewURL]);

  const cancelAvatarUpload = () => {
    revokeAvatarDraft(avatarDraft);
    setAvatarDraft(null);
    setAvatarEditorOpen(false);
  };

  const assignReport = async () => {
    if (!activeReport) return;
    setError("");
    setMessage("");
    try {
      await apiPatch(`/api/admin/reports/${activeReport.report_id}/assign`, token, {});
      setMessage("Case assigned to you.");
      loadReports();
    } catch (err) {
      setError(err.message);
    }
  };

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
      if (avatarDraft) {
        const croppedPhoto = await cropAvatarDraft(avatarDraft);
        if (croppedPhoto) body.append("profile_photo", croppedPhoto);
      }
      const updated = await apiMultipart("/api/customer/profile", token, body);
      setProfile((current) => ({ ...(current || {}), ...updated }));
      revokeAvatarDraft(avatarDraft);
      setAvatarDraft(null);
      setAvatarEditorOpen(false);
      setMessage("Profile saved.");
    } catch (err) {
      setError(err.message);
    }
  };

  const photoURL = avatarDraft?.previewURL || (profile?.profile_photo_url ? apiURL(profile.profile_photo_url) : "");
  const avatarStyle = avatarDraft ? {
    objectPosition: `${avatarDraft.x}% ${avatarDraft.y}%`,
    transform: `scale(${avatarDraft.zoom})`,
    transformOrigin: `${avatarDraft.x}% ${avatarDraft.y}%`,
  } : undefined;
  const customerName = profile?.full_name || profile?.customer_name || profile?.name || "Customer";
  const savedAddressLabel = form.address || "No saved address yet";

  return (
    <section className="pagePanel profilePage customerProfilePage">
      <SectionHeader title="Customer profile" text="Photo, address and booking preferences." />
      <div className="customerProfileShell">
        <section className="profileHeroCard customerProfileHero">
          <div className="profileIdentity">
            <div className="profilePhoto">
              <span>WM</span>
              {photoURL ? <img src={photoURL} alt="" style={avatarStyle} onError={(event) => event.currentTarget.remove()} /> : null}
            </div>
            <div>
              <span className="profileRoleBadge">Customer</span>
              <h3>{customerName}</h3>
              <p>Keep your address and booking notes ready for workers.</p>
            </div>
          </div>
          <label className="fileButton profileUploadButton">
            Upload photo
            <input
              type="file"
              accept="image/png,image/jpeg,image/webp"
              onChange={(e) => {
                const nextFile = e.target.files?.[0] || null;
                if (!nextFile) return;
                revokeAvatarDraft(avatarDraft);
                setAvatarDraft(makeAvatarDraft(nextFile));
                setAvatarEditorOpen(true);
              }}
            />
          </label>
          {avatarDraft && (
            <div className="avatarDraftActions">
              <span className="muted">{avatarDraft.file.name}</span>
              <button className="secondaryButton" type="button" onClick={() => setAvatarEditorOpen(true)}>Edit crop</button>
            </div>
          )}
        </section>
        <form className="profileEditorCard customerProfileForm" onSubmit={submit}>
          <div className="profileFormHeader">
            <div>
              <h3>Booking preferences</h3>
              <p>Notes here help workers arrive prepared and avoid extra messages.</p>
            </div>
            <button type="button" className="secondaryButton profileLocationButton" onClick={useCurrentLocation}>Use current location</button>
          </div>
          <Field label="About me" light>
            <textarea value={form.bio} onChange={(e) => setForm({ ...form, bio: e.target.value })} placeholder="Add notes for workers: entrance, preferred contact, timing..." />
          </Field>
          {avatarEditorOpen && (
            <AvatarCropper
              draft={avatarDraft}
              onChange={setAvatarDraft}
              onClose={() => setAvatarEditorOpen(false)}
              onCancel={cancelAvatarUpload}
            />
          )}
          <div className="profileAddressRow">
            <Field label="Saved address" light>
              <input value={form.address} onChange={(e) => setForm({ ...form, address: e.target.value })} placeholder="Street, building, entrance" />
            </Field>
            <div className="profileAddressPreview">
              <small>Current saved address</small>
              <strong title={savedAddressLabel}>{savedAddressLabel}</strong>
            </div>
          </div>
          <button className="profileSaveButton">Save profile</button>
        </form>
      </div>
      <div className="profileLinks profileShortcutGrid">
        <button className="profileLinkCard" type="button" onClick={() => onNavigate("bookings")}>
          <span className="profileShortcutIcon" aria-hidden="true">B</span>
          <span>
            <strong>My bookings</strong>
            <small>Open all customer bookings</small>
          </span>
        </button>
        <button className="profileLinkCard" type="button" onClick={() => onNavigate("bookings")}>
          <span className="profileShortcutIcon" aria-hidden="true">R</span>
          <span>
            <strong>Requests history</strong>
            <small>Shown inside bookings</small>
          </span>
        </button>
        <button className="profileLinkCard" type="button" onClick={() => onNavigate("find")}>
          <span className="profileShortcutIcon" aria-hidden="true">M</span>
          <span>
            <strong>Find worker</strong>
            <small>Back to map search</small>
          </span>
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
  const [avatarDraft, setAvatarDraft] = useState(null);
  const [avatarEditorOpen, setAvatarEditorOpen] = useState(false);
  const [identityFile, setIdentityFile] = useState(null);
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

  useEffect(() => () => revokeAvatarDraft(avatarDraft), [avatarDraft?.previewURL]);

  const cancelAvatarUpload = () => {
    revokeAvatarDraft(avatarDraft);
    setAvatarDraft(null);
    setAvatarEditorOpen(false);
  };

  const stats = useMemo(() => buildIncomeStats(bookings), [bookings]);

  const submit = async (event) => {
    event.preventDefault();
    setError("");
    setMessage("");
    try {
      const body = new FormData();
      body.append("bio", form.bio);
      if (avatarDraft) {
        const croppedPhoto = await cropAvatarDraft(avatarDraft);
        if (croppedPhoto) body.append("profile_photo", croppedPhoto);
      }
      const updated = await apiMultipart("/api/worker/profile", token, body);
      setProfile((current) => ({ ...(current || {}), ...updated }));
      revokeAvatarDraft(avatarDraft);
      setAvatarDraft(null);
      setAvatarEditorOpen(false);
      setMessage("Profile saved.");
    } catch (err) {
      setError(err.message);
    }
  };

  const uploadIdentityDocument = async () => {
    if (!identityFile) {
      setError("Choose an identity document first.");
      return;
    }
    setError("");
    setMessage("");
    try {
      const body = new FormData();
      body.append("identity_document", identityFile);
      const doc = await apiMultipart("/api/worker/identity-document", token, body);
      setProfile((current) => ({ ...(current || {}), identity_document: doc, verification_status: "pending" }));
      setIdentityFile(null);
      setMessage("Identity document sent. Admin will verify it before you can go online.");
    } catch (err) {
      setError(err.message);
    }
  };

  const photoURL = avatarDraft?.previewURL || (profile?.profile_photo_url ? apiURL(profile.profile_photo_url) : "");
  const avatarStyle = avatarDraft ? {
    objectPosition: `${avatarDraft.x}% ${avatarDraft.y}%`,
    transform: `scale(${avatarDraft.zoom})`,
    transformOrigin: `${avatarDraft.x}% ${avatarDraft.y}%`,
  } : undefined;
  const workerName = profile?.worker_name || profile?.full_name || profile?.name || "Worker";
  const verifiedSkills = profile?.verified_skills || [];
  const identityDocument = profile?.identity_document;

  return (
    <section className="pagePanel profilePage workerProfilePage">
      <SectionHeader title="Worker profile" text="Profile photo, bio, verified skills and income analytics." />
      <div className="workerDashboardHero">
        <section className="profileHeroCard workerIdentityCard">
          <div className="profileIdentity">
            <div className="profilePhoto">
              <span>WM</span>
              {photoURL ? <img src={photoURL} alt="" style={avatarStyle} onError={(event) => event.currentTarget.remove()} /> : null}
            </div>
            <div>
              <span className="profileRoleBadge">Worker</span>
              <h3>{workerName}</h3>
              <p>{verifiedSkills.length} verified services ready for customer bookings.</p>
            </div>
          </div>
          <label className="fileButton profileUploadButton">
            Upload photo
            <input
              type="file"
              accept="image/png,image/jpeg,image/webp"
              onChange={(e) => {
                const nextFile = e.target.files?.[0] || null;
                if (!nextFile) return;
                revokeAvatarDraft(avatarDraft);
                setAvatarDraft(makeAvatarDraft(nextFile));
                setAvatarEditorOpen(true);
              }}
            />
          </label>
          {avatarDraft && (
            <div className="avatarDraftActions">
              <span className="muted">{avatarDraft.file.name}</span>
              <button className="secondaryButton" type="button" onClick={() => setAvatarEditorOpen(true)}>Edit crop</button>
            </div>
          )}
        </section>
        <form className="profileEditorCard workerBioCard" onSubmit={submit}>
          <div className="profileFormHeader">
            <div>
              <h3>Public bio</h3>
              <p>Tell customers where you work, how you approach jobs and what you do best.</p>
            </div>
            <button className="profileSaveButton">Save profile</button>
          </div>
          <Field label="About me" light>
            <textarea value={form.bio} onChange={(e) => setForm({ ...form, bio: e.target.value })} placeholder="Tell customers about your experience, approach and city." />
          </Field>
          {avatarEditorOpen && (
            <AvatarCropper
              draft={avatarDraft}
              onChange={setAvatarDraft}
              onClose={() => setAvatarEditorOpen(false)}
              onCancel={cancelAvatarUpload}
            />
          )}
        </form>
      </div>
      <div className="profileStatsGrid workerKpiGrid">
        <StatCard title="This week" value={formatMoney(stats.weekTotal) + " KZT"} text={stats.weekCompleted + " completed jobs"} />
        <StatCard title="This month" value={formatMoney(stats.monthTotal) + " KZT"} text={stats.monthCompleted + " completed jobs"} />
        <StatCard title="Rating" value={renderStars(profile?.rating || 0)} text={`${Number(profile?.rating || 0).toFixed(1)} average from customer reviews`} />
        <StatCard title="Average check" value={formatMoney(stats.average) + " KZT"} text="Completed jobs this month" />
      </div>
      <section className="profileSection incomeSection">
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
      <section className="profileSection skillsOverviewSection">
        <div className="sectionTitleRow">
          <h3>Verified skills</h3>
          <button className="secondaryButton fitButton" onClick={() => onNavigate("skills")}>Add service</button>
        </div>
        <div className="verifiedSkillsGrid">
          {verifiedSkills.length === 0 && <EmptyState title="No verified skills yet" text="Add a service and attach qualification evidence." />}
          {verifiedSkills.map((skill) => (
            <article className="verifiedSkillCard" key={skill.worker_skill_id}>
              <div>
                <strong>{categoryTitle(skill.category_name)}</strong>
                <span className="verifiedBadge">Verified</span>
              </div>
              <span>{String(skill.experience_level || "level").replace("_", " ")}</span>
              <small>Price agreed in chat</small>
            </article>
          ))}
        </div>
      </section>
      <section className="profileSection identityDocumentSection">
        <div className="sectionTitleRow">
          <h3>Identity verification</h3>
          <span className={`statusPill ${identityDocument?.status || "missing"}`}>{identityDocument?.status || "not uploaded"}</span>
        </div>
        <p className="muted">Upload an ID card or passport. Only admin and manager accounts can view this file.</p>
        {identityDocument?.file_name && (
          <span className="muted">Last uploaded: {identityDocument.file_name}</span>
        )}
        <div className="identityUploadRow">
          <label className="fileButton skillEvidenceButton">
            <span aria-hidden="true">+</span>
            Attach ID document
            <input
              type="file"
              accept="image/png,image/jpeg,image/webp,application/pdf"
              onChange={(event) => setIdentityFile(event.target.files?.[0] || null)}
            />
          </label>
          <span className="muted">{identityFile ? identityFile.name : "No file selected"}</span>
          <button className="secondaryButton" type="button" onClick={uploadIdentityDocument} disabled={!identityFile}>Send for review</button>
        </div>
      </section>
      <div className="profileLinks profileShortcutGrid">
        <button className="profileLinkCard" type="button" onClick={() => onNavigate("jobs")}>
          <span className="profileShortcutIcon" aria-hidden="true">J</span>
          <span>
            <strong>My jobs</strong>
            <small>Open assigned bookings</small>
          </span>
        </button>
        <button className="profileLinkCard" type="button" onClick={() => onNavigate("pro")}>
          <span className="profileShortcutIcon" aria-hidden="true">M</span>
          <span>
            <strong>Map</strong>
            <small>Return to online mode and job search</small>
          </span>
        </button>
        <button className="profileLinkCard" type="button" onClick={() => onNavigate("skills")}>
          <span className="profileShortcutIcon" aria-hidden="true">S</span>
          <span>
            <strong>Services</strong>
            <small>Manage verified skills</small>
          </span>
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
    <section className={compact ? "paymentGateSection" : "profilePaymentCard"}>
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
  const [workerProfile, setWorkerProfile] = useState(null);
  const [profileID, setProfileID] = useState("");
  const [form, setForm] = useState({ category_id: "", experience_level: "junior", price: "", evidence_note: "" });
  const [upgradeForm, setUpgradeForm] = useState({ worker_skill_id: "", requested_experience_level: "middle", evidence_note: "" });
  const [files, setFiles] = useState([]);
  const [upgradeFiles, setUpgradeFiles] = useState([]);
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
      .then((data) => {
        setWorkerProfile(data);
        setProfileID(data.worker_profile_id || "");
      })
      .catch(() => {
        setWorkerProfile(null);
        setProfileID("");
      });
  }, [token]);

  const submit = async (event) => {
    event.preventDefault();
    setError("");
    setMessage("");
    try {
      if (!profileID) {
        const createdProfile = await apiPost("/api/worker/profile", token, { bio: "" });
        const nextProfileID = createdProfile.worker_profile_id || createdProfile.id || "created";
        setProfileID(nextProfileID);
        setWorkerProfile((current) => ({ ...(current || {}), ...createdProfile, worker_profile_id: nextProfileID }));
      }
      const body = new FormData();
      body.append("category_id", form.category_id);
      body.append("experience_level", form.experience_level);
      body.append("price_base", form.price || "0");
      body.append("evidence_note", form.evidence_note);
      files.forEach((file) => body.append("evidence_files", file));
      await apiMultipart("/api/worker/skills", token, body);
      try {
        const refreshedProfile = await apiGet("/api/worker/profile", token);
        setWorkerProfile(refreshedProfile);
        setProfileID(refreshedProfile.worker_profile_id || "");
      } catch {
        // The service request was already created; profile refresh is only for the summary cards.
      }
      setFiles([]);
      setMessage("Service request sent. Admin will review qualification evidence.");
    } catch (err) {
      setError(err.message);
    }
  };

  const selectedCategory = categories.find((category) => String(category.category_id) === String(form.category_id));
  const verifiedSkills = workerProfile?.verified_skills || [];
  const profileStatus = workerProfile?.verification_status || (profileID ? "pending" : "not created");
  const selectedSummary = selectedCategory ? `${categoryTitle(selectedCategory.name)} / ${form.experience_level}` : "Choose a category and level";
  const selectedUpgradeSkill = verifiedSkills.find((skill) => String(skill.worker_skill_id) === String(upgradeForm.worker_skill_id));
  const upgradeLevels = nextSkillLevels(selectedUpgradeSkill?.experience_level);

  const submitUpgrade = async (event) => {
    event.preventDefault();
    setError("");
    setMessage("");
    try {
      if (!upgradeForm.worker_skill_id) {
        throw new Error("Choose a verified service first.");
      }
      if (upgradeFiles.length === 0) {
        throw new Error("Attach evidence for the upgrade.");
      }
      const body = new FormData();
      body.append("worker_skill_id", upgradeForm.worker_skill_id);
      body.append("requested_experience_level", upgradeForm.requested_experience_level);
      body.append("evidence_note", upgradeForm.evidence_note);
      upgradeFiles.forEach((file) => body.append("evidence_files", file));
      await apiMultipart("/api/worker/skill-upgrades", token, body);
      setUpgradeFiles([]);
      setUpgradeForm({ worker_skill_id: "", requested_experience_level: "middle", evidence_note: "" });
      setMessage("Upgrade request sent. Admin will review new evidence.");
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="pagePanel skillsPage">
      <SectionHeader title="Services" text="Choose a category, level and attach qualification evidence. Price is agreed in chat for each booking." />
      <article className="skillVerificationBanner">
        <span className="skillVerificationIcon" aria-hidden="true">i</span>
        <div>
          <strong>Verification required</strong>
          <p>Attach certificates, work photos, diplomas or portfolio files. Admin approval is required before going online.</p>
        </div>
        <span className="skillVerificationStatus">{profileStatus}</span>
      </article>
      <form className="skillForm" onSubmit={submit}>
        <div className="skillFormTop">
          <Field label="Service category" light>
            <select value={form.category_id} onChange={(e) => setForm({ ...form, category_id: e.target.value })} required>
              {categories.length === 0 && <option value="">Categories are not loaded</option>}
              {categories.map((category) => <option key={category.category_id} value={category.category_id}>{categoryTitle(category.name)}</option>)}
            </select>
          </Field>
          <div className="field light skillLevelField">
            <span>Level</span>
            <div className="segmentedControl">
              {["junior", "middle", "senior"].map((level) => (
                <button key={level} type="button" className={form.experience_level === level ? "active" : ""} onClick={() => setForm({ ...form, experience_level: level })}>{level}</button>
              ))}
            </div>
          </div>
          <div className="field light skillEvidenceField">
            <span>Qualification evidence</span>
            <label className="fileButton skillEvidenceButton">
              <span aria-hidden="true">+</span>
              Attach evidence
              <input type="file" multiple accept="image/png,image/jpeg,image/webp,application/pdf" onChange={(e) => setFiles(Array.from(e.target.files || []))} />
            </label>
            <div className="selectedFiles">
              {files.length === 0 ? <span>No files selected</span> : files.map((file) => <span key={file.name}>{file.name}</span>)}
            </div>
          </div>
        </div>
        <Field label="Admin note" light>
          <textarea value={form.evidence_note} onChange={(e) => setForm({ ...form, evidence_note: e.target.value })} placeholder="Example: 3 years of experience, certificate attached, recent work photos..." />
        </Field>
        <div className="skillFormFooter">
          <p>{selectedSummary}</p>
          <button className="skillSubmitButton">Add service</button>
        </div>
      </form>
      <section className="skillServicesPanel">
        <div className="sectionTitleRow">
          <h3>Your services</h3>
          <span>{verifiedSkills.length} verified</span>
        </div>
        <div className="workerServiceSummaryGrid">
          <article className="workerServiceSummaryCard pending">
            <strong>Pending review</strong>
            <span>New services stay hidden from customers until admin approves the evidence.</span>
          </article>
          {verifiedSkills.length === 0 ? (
            <EmptyState title="No verified services yet" text="Add a service with evidence, then it will appear here after approval." />
          ) : verifiedSkills.map((skill) => (
            <article className="workerServiceSummaryCard" key={skill.worker_skill_id || `${skill.category_name}-${skill.experience_level}`}>
              <strong>{categoryTitle(skill.category_name || skill.name || "Service")}</strong>
              <span>{skill.experience_level || "level not set"}</span>
              {nextSkillLevels(skill.experience_level).length > 0 && (
                <button
                  type="button"
                  className="linkButton"
                  onClick={() => setUpgradeForm({
                    worker_skill_id: String(skill.worker_skill_id),
                    requested_experience_level: nextSkillLevels(skill.experience_level)[0],
                    evidence_note: "",
                  })}
                >
                  Request upgrade
                </button>
              )}
            </article>
          ))}
        </div>
      </section>
      {verifiedSkills.length > 0 && (
        <form className="skillUpgradeForm" onSubmit={submitUpgrade}>
          <div>
            <h3>Upgrade a verified service</h3>
            <p>Choose an existing skill and attach fresh evidence. Your current level stays active while staff reviews the request.</p>
          </div>
          <div className="skillFormTop">
            <Field label="Verified service" light>
              <select
                value={upgradeForm.worker_skill_id}
                onChange={(e) => {
                  const nextSkill = verifiedSkills.find((skill) => String(skill.worker_skill_id) === e.target.value);
                  const levels = nextSkillLevels(nextSkill?.experience_level);
                  setUpgradeForm({
                    ...upgradeForm,
                    worker_skill_id: e.target.value,
                    requested_experience_level: levels[0] || "middle",
                  });
                }}
                required
              >
                <option value="">Choose service</option>
                {verifiedSkills.filter((skill) => nextSkillLevels(skill.experience_level).length > 0).map((skill) => (
                  <option key={skill.worker_skill_id} value={skill.worker_skill_id}>
                    {categoryTitle(skill.category_name || skill.name)} - {skill.experience_level}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="Requested level" light>
              <select
                value={upgradeForm.requested_experience_level}
                onChange={(e) => setUpgradeForm({ ...upgradeForm, requested_experience_level: e.target.value })}
                disabled={upgradeLevels.length === 0}
                required
              >
                {upgradeLevels.length === 0 && <option value="">No upgrade available</option>}
                {upgradeLevels.map((level) => <option key={level} value={level}>{level}</option>)}
              </select>
            </Field>
            <div className="field light skillEvidenceField">
              <span>Upgrade evidence</span>
              <label className="fileButton skillEvidenceButton">
                <span aria-hidden="true">+</span>
                Attach files
                <input type="file" multiple accept="image/png,image/jpeg,image/webp,application/pdf" onChange={(e) => setUpgradeFiles(Array.from(e.target.files || []))} />
              </label>
              <div className="selectedFiles">
                {upgradeFiles.length === 0 ? <span>No files selected</span> : upgradeFiles.map((file) => <span key={file.name}>{file.name}</span>)}
              </div>
            </div>
          </div>
          <Field label="Admin note" light>
            <textarea value={upgradeForm.evidence_note} onChange={(e) => setUpgradeForm({ ...upgradeForm, evidence_note: e.target.value })} placeholder="What changed since the last verification?" />
          </Field>
          <button className="secondaryButton">Send upgrade request</button>
        </form>
      )}
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
    : `Следуйте по маршруту примерно ${formatNavigationDistance(meters)}.`;
  speakText(text);
}

function speakText(text) {
  if (!("speechSynthesis" in window)) {
    return;
  }
  const utterance = new SpeechSynthesisUtterance(text);
  utterance.lang = "ru-RU";
  const voice = preferredRussianVoice();
  if (voice) {
    utterance.voice = voice;
  }
  utterance.rate = 0.9;
  utterance.pitch = 1.05;
  window.speechSynthesis.cancel();
  window.speechSynthesis.speak(utterance);
}

function preferredRussianVoice() {
  if (!("speechSynthesis" in window)) {
    return null;
  }
  const voices = window.speechSynthesis.getVoices?.() || [];
  const hint = TTS_VOICE_HINT.trim().toLowerCase();
  if (hint) {
    const hintedVoice = voices.find((voice) => voice.name.toLowerCase().includes(hint) || voice.lang.toLowerCase().includes(hint));
    if (hintedVoice) {
      return hintedVoice;
    }
  }
  return voices.find((voice) => /ru/i.test(voice.lang) && /natural|online|google|microsoft|irina|milena|dariya|svetlana|female/i.test(voice.name)) ||
    voices.find((voice) => /ru/i.test(voice.lang)) ||
    null;
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

function shouldRefreshRoute(cache, key, start) {
  if (!cache || cache.key !== key || !cache.start) {
    return true;
  }
  const elapsed = Date.now() - Number(cache.at || 0);
  if (elapsed < ROUTE_REFRESH_MS) {
    return false;
  }
  return haversineMeters(cache.start, start) >= ROUTE_REFRESH_DISTANCE_M;
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

function submitTextareaOnEnter(event) {
  if (event.key !== "Enter" || event.shiftKey || event.isComposing) {
    return;
  }
  event.preventDefault();
  event.currentTarget.form?.requestSubmit();
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
