import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { apiGet, apiPatch, apiPost } from "./api.js";
import MapView from "./MapView.jsx";
import WorkerList from "./WorkerList.jsx";
import { useGeolocation } from "./useGeolocation.js";

const TOKEN_KEY = "workers_marketplace_token";
const ROLE_KEY = "workers_marketplace_role";

const roleTabs = {
  customer: [
    ["find", "Find worker"],
    ["requests", "Requests"],
    ["bookings", "Bookings"],
    ["profile", "Profile"],
    ["notifications", "Alerts"],
  ],
  worker: [
    ["pro", "Pro"],
    ["jobs", "Jobs"],
    ["skills", "Skills"],
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
  const [mode, setMode] = useState("signin");
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
  const [reset, setReset] = useState({ token: "", new_password: "" });
  const [verifyToken, setVerifyToken] = useState("");

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
      setMode("signin");
    });
  };

  const submitForgot = (event) => {
    event.preventDefault();
    run(async () => {
      await apiPost("/auth/forgot-password", "", { email: forgotEmail });
      setMessage("Password reset link/token was sent if the email exists.");
    });
  };

  const submitReset = (event) => {
    event.preventDefault();
    run(async () => {
      await apiPost("/auth/reset-password", "", reset);
      setMessage("Password was reset. You can sign in now.");
      setMode("signin");
    });
  };

  const submitVerify = (event) => {
    event.preventDefault();
    run(async () => {
      await apiGet(`/auth/verify?token=${encodeURIComponent(verifyToken)}`, "");
      setMessage("Email verified. You can sign in now.");
      setMode("signin");
    });
  };

  return (
    <main className="authShell">
      <section className="authIntro">
        <div className="appIcon">WM</div>
        <h1>Workers Marketplace</h1>
        <p>Sign in first. Then the app opens either the customer flow for hiring or the worker Pro flow for accepting jobs.</p>
      </section>
      <section className="authCard">
        <div className="authTabs">
          <button className={mode === "signin" ? "active" : ""} onClick={() => setMode("signin")}>Sign in</button>
          <button className={mode === "signup" ? "active" : ""} onClick={() => setMode("signup")}>Sign up</button>
          <button className={mode === "forgot" ? "active" : ""} onClick={() => setMode("forgot")}>Reset</button>
          <button className={mode === "verify" ? "active" : ""} onClick={() => setMode("verify")}>Verify</button>
        </div>

        {mode === "signin" && (
          <form className="formStack" onSubmit={submitLogin}>
            <Field label="Email"><input value={login.email} onChange={(e) => setLogin({ ...login, email: e.target.value })} type="email" required /></Field>
            <Field label="Password"><input value={login.password} onChange={(e) => setLogin({ ...login, password: e.target.value })} type="password" required /></Field>
            <button disabled={busy}>Sign in</button>
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
          <div className="formStack">
            <form className="formStack" onSubmit={submitForgot}>
              <Field label="Email"><input value={forgotEmail} onChange={(e) => setForgotEmail(e.target.value)} type="email" required /></Field>
              <button disabled={busy}>Send reset token</button>
            </form>
            <form className="formStack" onSubmit={submitReset}>
              <Field label="Reset token"><input value={reset.token} onChange={(e) => setReset({ ...reset, token: e.target.value })} required /></Field>
              <Field label="New password"><input value={reset.new_password} onChange={(e) => setReset({ ...reset, new_password: e.target.value })} type="password" required /></Field>
              <button className="secondaryButton" disabled={busy}>Set new password</button>
            </form>
          </div>
        )}

        {mode === "verify" && (
          <form className="formStack" onSubmit={submitVerify}>
            <Field label="Email verification token"><input value={verifyToken} onChange={(e) => setVerifyToken(e.target.value)} required /></Field>
            <button disabled={busy}>Verify email</button>
          </form>
        )}

        <Messages message={message} error={error} />
      </section>
    </main>
  );
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
  const { position, geoStatus, geoError, locate } = useGeolocation();
  const mapRef = useRef(null);
  const [available, setAvailable] = useState(false);
  const [searching, setSearching] = useState(false);
  const [bookings, setBookings] = useState([]);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const loadBookings = useCallback(async () => {
    setSearching(true);
    setError("");
    try {
      const data = await apiGet("/api/bookings/my", token);
      setBookings(Array.isArray(data) ? data : data.bookings || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setSearching(false);
    }
  }, [token]);

  useEffect(() => {
    locate();
    loadBookings();
  }, [locate, loadBookings]);

  const syncLocation = async () => {
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
  };

  const toggleAvailability = async () => {
    setError("");
    try {
      const next = !available;
      await apiPatch("/api/worker/availability", token, { is_available: next });
      setAvailable(next);
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

  return (
    <div className="proPhoneShell">
      <section className="proPhone" aria-label="Worker Pro map workspace">
        <MapView ref={mapRef} position={position} workers={[]} selectedWorker={null} onSelectWorker={() => {}} />
        <div className="phoneStatusBar">
          <span>12:30</span>
          <span className="phoneSignal" />
        </div>
        <WorkerPhoneTabs activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut} />
        <div className="proScoreBubble">
          <strong>{available ? "8" : "6"}</strong>
        </div>
        <strong className="proSearchLabel">{available ? "online" : "search"}</strong>
        <div className="proMoneyCard">
          <strong>{bookings.length > 0 ? `${bookings.length * 2800},25 KZT` : "0 KZT"}</strong>
          <span>{searching ? "searching" : `${bookings.length} jobs`}</span>
          <div className="proAvatar">WM</div>
        </div>
        <button className="roundMapButton searchButton" onClick={loadBookings}>⌕</button>
        <button className="roundMapButton plusButton" onClick={() => mapRef.current?.zoomIn()}>+</button>
        <button className="roundMapButton minusButton" onClick={() => mapRef.current?.zoomOut()}>−</button>
        <button className="roundMapButton navButtonMap" onClick={() => { locate(); mapRef.current?.recenter(); }}>⌖</button>
        <div className="energyPill">
          <strong>↯</strong>
          <span>2,2</span>
          <span>...</span>
          <span>3,4</span>
        </div>
        <div className="driverArrow" />
        <button className={available ? "lineButton online" : "lineButton"} onClick={toggleAvailability}>
          {available ? "Go offline" : "Go online"}
        </button>
        <div className="offersDrawer">
          <div>
            <h2>Offers</h2>
            <button className="walletButton" onClick={() => onNavigate("jobs")}>▣</button>
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
        <div className="phoneStatusBar">
          <span>12:30</span>
          <span className="phoneSignal" />
        </div>
        <WorkerPhoneTabs activeTab={activeTab} onNavigate={onNavigate} onSignOut={onSignOut} />
        <div className="workerInnerPage">{children}</div>
      </section>
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

function AdminApp({ token, activeTab }) {
  const [overview, setOverview] = useState(null);
  const [workerID, setWorkerID] = useState("");
  const [skillID, setSkillID] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    if (activeTab === "overview") {
      apiGet("/api/admin/overview", token).then(setOverview).catch((err) => setError(err.message));
    }
  }, [activeTab, token]);

  if (activeTab === "notifications") return <NotificationsPanel token={token} />;

  const verify = async (type) => {
    setError("");
    try {
      if (type === "worker") {
        await apiPost("/api/admin/verify-worker", token, { worker_profile_id: Number(workerID) });
      } else {
        await apiPost("/api/admin/verify-skill", token, { worker_skill_id: Number(skillID) });
      }
      setMessage("Verification request completed.");
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="pagePanel">
      <SectionHeader title={activeTab === "verify" ? "Verification" : "Admin overview"} text="Administrative controls for the marketplace." />
      {activeTab === "verify" ? (
        <div className="splitGrid">
          <div className="toolCard">
            <h3>Verify worker</h3>
            <Field label="Worker profile ID" light><input value={workerID} onChange={(e) => setWorkerID(e.target.value)} /></Field>
            <button onClick={() => verify("worker")}>Verify worker</button>
          </div>
          <div className="toolCard">
            <h3>Verify skill</h3>
            <Field label="Worker skill ID" light><input value={skillID} onChange={(e) => setSkillID(e.target.value)} /></Field>
            <button onClick={() => verify("skill")}>Verify skill</button>
          </div>
        </div>
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
  const [form, setForm] = useState({ bio: "", current_latitude: "", current_longitude: "" });
  return (
    <ProfileForm
      title="Worker profile"
      text="Describe yourself and set your working location."
      form={form}
      setForm={setForm}
      links={[
        ["jobs", "My bookings", "Open assigned jobs"],
        ["pro", "Go Pro", "Back to map and online status"],
        ["skills", "Skills", "Manage services and prices"],
      ]}
      onNavigate={onNavigate}
      onSubmit={() => apiPost("/api/worker/profile", token, {
        bio: form.bio,
        current_latitude: Number(form.current_latitude),
        current_longitude: Number(form.current_longitude),
      })}
    />
  );
}

function WorkerSkillsPanel({ token }) {
  const [form, setForm] = useState({ category_id: "", experience_level: "junior", price: "" });
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const submit = async (event) => {
    event.preventDefault();
    setError("");
    try {
      await apiPost("/api/worker/skills", token, {
        category_id: Number(form.category_id),
        experience_level: form.experience_level,
        price: Number(form.price),
      });
      setMessage("Skill sent for verification.");
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="pagePanel">
      <SectionHeader title="Skills" text="Add worker categories and prices." />
      <form className="formGrid" onSubmit={submit}>
        <Field label="Category ID" light><input value={form.category_id} onChange={(e) => setForm({ ...form, category_id: e.target.value })} required /></Field>
        <Field label="Experience" light><input value={form.experience_level} onChange={(e) => setForm({ ...form, experience_level: e.target.value })} required /></Field>
        <Field label="Price KZT" light><input value={form.price} onChange={(e) => setForm({ ...form, price: e.target.value })} required /></Field>
        <button>Add skill</button>
      </form>
      <Messages message={message} error={error} />
    </section>
  );
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
