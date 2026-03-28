import { useEffect, useMemo, useState } from "react";
import PageHeader from "../components/PageHeader";
import { useAuth } from "../context/AuthContext";
import { apiRequest, extractErrorMessage } from "../lib/api";

export default function WorkerProfilePage() {
  const { token, userId } = useAuth();

  const storageKey = useMemo(
    () => `wm_worker_available_${userId || "default"}`,
    [userId],
  );
  const [available, setAvailable] = useState(false);
  const [loadingProfile, setLoadingProfile] = useState(false);
  const [loadingAvailability, setLoadingAvailability] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    const raw = localStorage.getItem(storageKey);
    if (raw !== null) {
      setAvailable(raw === "true");
    }
  }, [storageKey]);

  const createProfile = async () => {
    setLoadingProfile(true);
    setError("");
    setMessage("");
    try {
      const data = await apiRequest("/api/worker/profile", {
        method: "POST",
        token,
      });
      setMessage(data?.message || "Worker profile created.");
    } catch (requestError) {
      const text = extractErrorMessage(requestError.data || requestError.message);
      if (text.toLowerCase().includes("duplicate")) {
        setMessage("Worker profile already exists.");
      } else {
        setError(text);
      }
    } finally {
      setLoadingProfile(false);
    }
  };

  const updateAvailability = async () => {
    setLoadingAvailability(true);
    setError("");
    setMessage("");
    try {
      const data = await apiRequest("/api/worker/availability", {
        method: "PATCH",
        token,
        body: { is_available: available },
      });
      localStorage.setItem(storageKey, String(available));
      setMessage(data?.message || "Availability updated.");
    } catch (requestError) {
      setError(extractErrorMessage(requestError.data || requestError.message));
    } finally {
      setLoadingAvailability(false);
    }
  };

  return (
    <section className="rounded-2xl border border-slate-200/70 bg-white/90 p-5 shadow-panel">
      <PageHeader
        title="Worker Profile"
        subtitle="Create profile first, then add skills and set availability."
      />

      <div className="flex flex-wrap gap-3">
        <button
          type="button"
          onClick={createProfile}
          disabled={loadingProfile}
          className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {loadingProfile ? "Creating..." : "Create Worker Profile"}
        </button>
      </div>

      <div className="mt-5 rounded-xl border border-slate-200 bg-white p-4">
        <label className="flex items-center gap-2 text-sm font-medium text-slate-700">
          <input
            type="checkbox"
            checked={available}
            onChange={(e) => setAvailable(e.target.checked)}
            className="h-4 w-4 rounded border-slate-300"
          />
          Is Available
        </label>
        <button
          type="button"
          onClick={updateAvailability}
          disabled={loadingAvailability}
          className="mt-3 rounded-lg border border-slate-300 px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {loadingAvailability ? "Saving..." : "Update Availability"}
        </button>
      </div>

      {message ? (
        <p className="mt-4 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
          {message}
        </p>
      ) : null}
      {error ? (
        <p className="mt-4 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
          {error}
        </p>
      ) : null}
    </section>
  );
}
