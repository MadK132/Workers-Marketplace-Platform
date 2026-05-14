import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { apiGet, apiMultipart, apiPatch, apiPost, apiURL } from "./api.js";
import MapView from "./MapView.jsx";
import WorkerList from "./WorkerList.jsx";
import { useGeolocation } from "./useGeolocation.js";

const TOKEN_KEY = "workers_marketplace_token";
const ROLE_KEY = "workers_marketplace_role";
const ASTANA_BOUNDS = {
  minLatitude: 50.95,
  maxLatitude: 51.35,
  minLongitude: 71.15,
  maxLongitude: 71.75,
};

const roleTabs = {
  customer: [
    ["find", "Search"],
    ["requests", "Requests"],
    ["bookings", "Bookings"],
    ["profile", "Profile"],
    ["notifications", "Alerts"],
  ],
  worker: [
    ["pro", "Map"],
    ["jobs", "Jobs"],
    ["skills", "Services"],
    ["profile", "Profile"],
    ["notifications", "Alerts"],
  ],
  admin: [
    ["overview", "Overview"],
    ["verify", "Verify"],
    ["notifications", "Alerts"],
  ],
};

export default function App() {
  const [token, setToken] = useState(() => localStorage.getItem(TOKEN_KEY) || "");
  const [role, setRole] = useState(() => localStorage.getItem(ROLE_KEY) || readRole(token));
  const [activeTab, setActiveTab] = useState(role === "worker" ? "pro" : "find");
  const session = useMemo(() => decodeToken(token), [token]);

  const saveSession = useCallback((nextToken, fallbackRole) => {
    const nextRole = readRole(nextToken) || fallbackRole || "";
    localStorage.setItem(TOKEN_KEY, nextToken);
    localStorage.setItem(ROLE_KEY, nextRole);
    setToken(nextToken);
    setRole(nextRole);
    setActiveTab(nextRole === "worker" ? "pro" : "find");
  }, []);

  const signOut = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(ROLE_KEY);
    clearAuthURL();
    setToken("");
    setRole("");
    setActiveTab("find");
  }, []);

  if (!token) {
    return <AuthScreen onAuth={saveSession} />;
  }

  return (
    <AppFrame role={role} session={session} activeTab={activeTab} onTab={setActiveTab} onSignOut={signOut}>
      {role === "customer" && <CustomerApp token={token} activeTab={activeTab} onNavigate={setActiveTab} />}
      {role === "worker" && <WorkerApp token={token} activeTab={activeTab} onNavigate={setActiveTab} onSignOut={signOut} />}
      {role === "admin" && <AdminApp token={token} activeTab={activeTab} />}
      {!["customer", "worker", "admin"].includes(role) && (
        <EmptyState title="Role is missing" text="Sign out and sign in again, or select a role in the backend." />
      )}
    </AppFrame>
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

function AppFrame({ role, session, activeTab, onTab, onSignOut, children }) {
  const tabs = roleTabs[role] || [];
  if (role === "worker") {
    return <main className="workerFullscreen">{children}</main>;
  }
  return (
    <main className="dashboardShell">
      <aside className="controlPanel dashboardNav">
        <div className="brandBlock">
          <div className="appIcon">WM</div>
          <div>
            <h1>{role === "worker" ? "Pro workspace" : "Marketplace"}</h1>
            <p>{session.email || "Signed in"} - {role || "no role"}</p>
          </div>
        </div>
        <nav className="sideNav">
          {tabs.map(([id, label]) => (
            <button key={id} className={activeTab === id ? "navButton active" : "navButton"} onClick={() => onTab(id)}>
              {label}
            </button>
          ))}
        </nav>
        <button className="ghostButton" onClick={onSignOut}>Sign out</button>
      </aside>
      <section className="dashboardBody">{children}</section>
    </main>
  );
}

function CustomerApp({ token, activeTab, onNavigate }) {
  const { position, geoStatus, geoError, locate } = useGeolocation();
  const [categories, setCategories] = useState([]);
  const [categoryID, setCategoryID] = useState("");
  const [radius, setRadius] = useState(5000);
  const [workers, setWorkers] = useState([]);
  const [selectedWorker, setSelectedWorker] = useState(null);
  const [description, setDescription] = useState("");
  const [address, setAddress] = useState("");
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    locate();
    apiGet("/api/categories", token).then(setCategories).catch((err) => setError(err.message));
  }, [locate, token]);

  const searchWorkers = async () => {
    if (!position || !categoryID) {
      setError("Choose category and allow location first.");
      return;
    }
    if (!isInsideAstana(position)) {
      setError("Service is available only in Astana.");
      return;
    }
    setLoading(true);
    setError("");
    setMessage("");
    try {
      const query = new URLSearchParams({
        category_id: categoryID,
        latitude: String(position.latitude),
        longitude: String(position.longitude),
        radius_meters: String(radius),
      });
      const data = await apiGet(`/api/geo/workers/nearby?${query}`, token);
      setWorkers(Array.isArray(data) ? data : data.workers || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const hireWorker = async (worker) => {
    if (!position || !categoryID) {
      setError("Location and category are required.");
      return;
    }
    if (!isInsideAstana(position)) {
      setError("Orders are accepted only in Astana.");
      return;
    }
    setError("");
    try {
      const request = await apiPost("/api/requests", token, {
        category_id: Number(categoryID),
        description: description || `Request for ${worker.full_name}`,
        address: address || "Customer location",
        latitude: position.latitude,
        longitude: position.longitude,
      });
      await apiPost("/api/bookings", token, {
        request_id: Number(request.request_id || request.id),
        worker_profile_id: Number(worker.worker_id || worker.worker_profile_id),
      });
      setMessage(`Booking request sent to ${worker.full_name}.`);
    } catch (err) {
      setError(err.message);
    }
  };

  if (activeTab === "requests") return <RequestsPanel token={token} />;
  if (activeTab === "bookings") return <BookingsPanel token={token} />;
  if (activeTab === "profile") return <CustomerProfilePanel token={token} onNavigate={onNavigate} />;
  if (activeTab === "notifications") return <NotificationsPanel token={token} />;

  return (
    <div className="customerGrid">
      <section className="mapArea">
        <MapView position={position} workers={workers} selectedWorker={selectedWorker} onSelectWorker={setSelectedWorker} />
      </section>
      <section className="pagePanel">
        <SectionHeader title="Find nearby worker" text="Customer interface: choose a category, search around your position, then pick a worker." />
        <div className="toolbarGrid">
          <Field label="Category" light>
            <select value={categoryID} onChange={(e) => setCategoryID(e.target.value)}>
              <option value="">Choose category</option>
              {categories.map((category) => (
                <option key={category.category_id} value={category.category_id}>{category.name}</option>
              ))}
            </select>
          </Field>
          <Field label="Radius" light>
            <select value={radius} onChange={(e) => setRadius(Number(e.target.value))}>
              <option value={1500}>1.5 km</option>
              <option value={5000}>5 km</option>
              <option value={10000}>10 km</option>
            </select>
          </Field>
          <button onClick={searchWorkers} disabled={loading}>Search</button>
          <button className="secondaryButton" onClick={locate}>Use my location</button>
        </div>
        <div className="formGrid">
          <Field label="Address" light><input value={address} onChange={(e) => setAddress(e.target.value)} placeholder="Where should worker arrive?" /></Field>
          <Field label="Description" light><input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Describe the task" /></Field>
        </div>
        <StatusLine geoStatus={geoStatus} geoError={geoError} />
        <Messages message={message} error={error} />
        <WorkerList workers={workers} selectedWorker={selectedWorker} onSelectWorker={setSelectedWorker} onHireWorker={hireWorker} loading={loading} />
      </section>
    </div>
  );
}

function WorkerApp({ token, activeTab, onNavigate, onSignOut }) {
  const { position, geoStatus, geoError, startWatch } = useGeolocation();
  const mapRef = useRef(null);
  const [available, setAvailable] = useState(false);
  const [searching, setSearching] = useState(false);
  const [bookings, setBookings] = useState([]);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const loadBookings = useCallback(async () => {
    setError("");
    try {
      const data = await apiGet("/api/bookings/my", token);
      const nextBookings = Array.isArray(data) ? data : data.bookings || [];
      setBookings(nextBookings);
      if (nextBookings.length > 0) {
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

  useEffect(() => {
    if (!available || !position) {
      return undefined;
    }
    syncLocation();
    const intervalID = window.setInterval(() => {
      syncLocation();
      if (searching) {
        loadBookings();
      }
    }, 15000);
    return () => window.clearInterval(intervalID);
  }, [available, loadBookings, position, searching, syncLocation]);

  const toggleAvailability = async () => {
    setError("");
    try {
      const next = !available;
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
      await apiPatch(`/api/bookings/${bookingID}/${action}`, token, {});
      setMessage(`Booking ${action}ed.`);
      loadBookings();
    } catch (err) {
      setError(err.message);
    }
  };

  if (activeTab === "jobs") {
    return <WorkerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><BookingsPanel token={token} canProgress onProgress={updateBooking} /></WorkerPhonePage>;
  }
  if (activeTab === "skills") {
    return <WorkerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><WorkerSkillsPanel token={token} /></WorkerPhonePage>;
  }
  if (activeTab === "profile") {
    return <WorkerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><WorkerProfilePanel token={token} onNavigate={onNavigate} /></WorkerPhonePage>;
  }
  if (activeTab === "notifications") {
    return <WorkerPhonePage activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut}><NotificationsPanel token={token} /></WorkerPhonePage>;
  }

  if (!position) {
    return <WorkerLocationGate geoStatus={geoStatus} geoError={geoError} onAllow={startWatch} onSignOut={onSignOut} />;
  }

  return (
    <div className="proPhoneShell">
      <section className="proPhone" aria-label="Worker Pro map workspace">
        <MapView ref={mapRef} position={position} workers={[]} selectedWorker={null} onSelectWorker={() => {}} userMarker="driver" />
        <WorkerPhoneTabs activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut} />
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
          <JobBoard bookings={bookings.slice(0, 2)} onProgress={updateBooking} compact />
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
  return (
    <main className="geoGate">
      <section className="geoGateCard">
        <div className="appIcon">WM</div>
        <h1>Allow location</h1>
        <p>We need your location for the worker map and online job search.</p>
        <button onClick={onAllow} disabled={geoStatus === "loading"}>{geoStatus === "loading" ? "Requesting..." : "Allow location"}</button>
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

function AdminApp({ token, activeTab }) {
  const [overview, setOverview] = useState(null);
  const [workerID, setWorkerID] = useState("");
  const [skillID, setSkillID] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const loadOverview = useCallback(() => {
    apiGet("/api/admin/overview", token).then(setOverview).catch((err) => setError(err.message));
  }, [token]);

  useEffect(() => {
    if (activeTab === "overview" || activeTab === "verify") {
      loadOverview();
    }
  }, [activeTab, loadOverview]);

  if (activeTab === "notifications") return <NotificationsPanel token={token} />;

  const verify = async (type) => {
    setError("");
    setMessage("");
    try {
      if (type === "worker") {
        await apiPost("/api/admin/verify-worker", token, { worker_id: Number(workerID) });
      } else {
        await apiPost("/api/admin/verify-skill", token, { worker_skill_id: Number(skillID) });
      }
      setMessage("Verification request completed.");
      loadOverview();
    } catch (err) {
      setError(err.message);
    }
  };

  const verifyWorker = async (id) => {
    setWorkerID(String(id));
    setError("");
    setMessage("");
    try {
      await apiPost("/api/admin/verify-worker", token, { worker_id: Number(id) });
      setMessage("Worker verified.");
      loadOverview();
    } catch (err) {
      setError(err.message);
    }
  };

  const verifySkill = async (id) => {
    setSkillID(String(id));
    setError("");
    setMessage("");
    try {
      await apiPost("/api/admin/verify-skill", token, { worker_skill_id: Number(id) });
      setMessage("Skill verified.");
      loadOverview();
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="pagePanel">
      <SectionHeader title={activeTab === "verify" ? "Verification" : "Admin overview"} text="Administrative controls for the marketplace." />
      {activeTab === "verify" ? (
        <AdminVerificationPanel
          overview={overview}
          workerID={workerID}
          skillID={skillID}
          setWorkerID={setWorkerID}
          setSkillID={setSkillID}
          verify={verify}
          verifyWorker={verifyWorker}
          verifySkill={verifySkill}
        />
      ) : (
        <pre className="jsonBox">{JSON.stringify(overview || {}, null, 2)}</pre>
      )}
      <Messages message={message} error={error} />
    </section>
  );
}

function RequestsPanel({ token }) {
  return <ListPanel title="My requests" endpoint="/api/requests/my" token={token} empty="No service requests yet." />;
}

function AdminVerificationPanel({
  overview,
  workerID,
  skillID,
  setWorkerID,
  setSkillID,
  verify,
  verifyWorker,
  verifySkill,
}) {
  const pendingWorkers = overview?.pending_workers || [];
  const pendingSkills = overview?.pending_skills || [];

  return (
    <div className="adminVerifyGrid">
      <div className="toolCard">
        <h3>Pending workers</h3>
        <p className="muted">Check worker identity/profile data and approve the profile.</p>
        <div className="dataList">
          {pendingWorkers.length === 0 && <EmptyState title="No pending workers" text="New worker profiles will appear here." />}
          {pendingWorkers.map((worker) => (
            <article className="dataRow" key={worker.worker_profile_id}>
              <strong>{worker.full_name}</strong>
              <span>{worker.email}</span>
              <span>Profile #{worker.worker_profile_id} - User #{worker.user_id}</span>
              <button onClick={() => verifyWorker(worker.worker_profile_id)}>Verify worker</button>
            </article>
          ))}
        </div>
      </div>
      <div className="toolCard">
        <h3>Pending skills</h3>
        <p className="muted">Approve skills after checking category, price and worker profile.</p>
        <div className="dataList">
          {pendingSkills.length === 0 && <EmptyState title="No pending skills" text="New worker skills will appear here." />}
          {pendingSkills.map((skill) => (
            <article className="dataRow" key={skill.worker_skill_id}>
              <strong>{categoryTitle(skill.category_name)}</strong>
              <span>{skill.worker_full_name} - {skill.worker_user_email}</span>
              <span>{skill.experience_level} - {skill.price_base} KZT - Skill #{skill.worker_skill_id}</span>
              <EvidenceLinks value={skill.evidence_files} />
              <button onClick={() => verifySkill(skill.worker_skill_id)}>Verify skill</button>
            </article>
          ))}
        </div>
      </div>
      <div className="toolCard">
        <h3>Manual verify</h3>
        <Field label="Worker profile ID" light><input value={workerID} onChange={(e) => setWorkerID(e.target.value)} /></Field>
        <button onClick={() => verify("worker")}>Verify worker</button>
        <Field label="Worker skill ID" light><input value={skillID} onChange={(e) => setSkillID(e.target.value)} /></Field>
        <button className="secondaryButton" onClick={() => verify("skill")}>Verify skill</button>
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

function BookingsPanel({ token, canProgress, onProgress }) {
  const [items, setItems] = useState([]);
  const [error, setError] = useState("");

  useEffect(() => {
    apiGet("/api/bookings/my", token).then((data) => setItems(Array.isArray(data) ? data : data.bookings || [])).catch((err) => setError(err.message));
  }, [token]);

  return (
    <section className="pagePanel">
      <SectionHeader title="Bookings" text="Current and past bookings." />
      <Messages error={error} />
      <div className="dataList">
        {items.length === 0 && <EmptyState title="No bookings" text="Bookings will appear here after customer selects a worker." />}
        {items.map((item) => (
          <article className="dataRow" key={item.booking_id || item.id}>
            <strong>Booking #{item.booking_id || item.id}</strong>
            <span>Status: {item.status || item.booking_status || "unknown"}</span>
            <small>{item.description || item.address || ""}</small>
            {canProgress && (
              <div className="rowActions">
                <button onClick={() => onProgress(item.booking_id || item.id, "start")}>Start</button>
                <button className="secondaryButton" onClick={() => onProgress(item.booking_id || item.id, "complete")}>Complete</button>
              </div>
            )}
          </article>
        ))}
      </div>
    </section>
  );
}

function NotificationsPanel({ token }) {
  const [items, setItems] = useState([]);
  const [error, setError] = useState("");

  const load = useCallback(() => {
    apiGet("/api/notifications", token).then((data) => setItems(Array.isArray(data) ? data : data.notifications || [])).catch((err) => setError(err.message));
  }, [token]);

  useEffect(() => load(), [load]);

  const markAll = async () => {
    await apiPatch("/api/notifications/read-all", token, {});
    load();
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
          </article>
        ))}
      </div>
    </section>
  );
}

function CustomerProfilePanel({ token, onNavigate }) {
  const [form, setForm] = useState({ address: "", latitude: "", longitude: "" });
  return (
    <ProfileForm
      title="Customer profile"
      text="Set the customer location used for searching and booking."
      form={form}
      setForm={setForm}
      links={[
        ["bookings", "My bookings", "Open all customer bookings"],
        ["requests", "My requests", "Track created service requests"],
        ["find", "Find worker", "Back to map search"],
      ]}
      onNavigate={onNavigate}
      onSubmit={() => apiPost("/api/customer/profile", token, {
        address: form.address,
        latitude: Number(form.latitude),
        longitude: Number(form.longitude),
      })}
    />
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
              <span>{skill.experience_level} - from {skill.price_base} KZT</span>
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
          <span>Manage skills and prices</span>
        </button>
      </div>
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
      body.append("price_base", form.price);
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
      <SectionHeader title="Services" text="Choose a category, level, base price and attach qualification evidence." />
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
        <Field label="Price from, KZT" light>
          <input value={form.price} onChange={(e) => setForm({ ...form, price: e.target.value })} inputMode="numeric" placeholder="15000" required />
        </Field>
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
        return (
          <article className="jobCard" key={id}>
            <div>
              <strong>Job #{id}</strong>
              <span>{booking.status || booking.booking_status || "pending"}</span>
            </div>
            <p>{booking.address || booking.description || "Customer task"}</p>
            <div className="rowActions">
              <button onClick={() => onProgress(id, "start")}>Start</button>
              <button className="secondaryButton" onClick={() => onProgress(id, "complete")}>Complete</button>
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
