import { useEffect, useState } from "react";
import PageHeader from "../components/PageHeader";
import { useAuth } from "../context/AuthContext";
import { apiRequest, extractErrorMessage } from "../lib/api";

export default function AdminOverviewPage() {
  const { token } = useAuth();

  const [overview, setOverview] = useState(null);
  const [loading, setLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState(null);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  const loadOverview = async () => {
    setLoading(true);
    setError("");
    try {
      const data = await apiRequest("/api/admin/overview", { token });
      setOverview(data);
    } catch (requestError) {
      setError(extractErrorMessage(requestError.data || requestError.message));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadOverview();
  }, []);

  const verifyWorker = async (workerId) => {
    setActionLoading(`worker-${workerId}`);
    setError("");
    setMessage("");
    try {
      const data = await apiRequest("/api/admin/verify-worker", {
        method: "POST",
        token,
        body: { worker_id: workerId },
      });
      setMessage(data?.message || "Worker verified.");
      await loadOverview();
    } catch (requestError) {
      setError(extractErrorMessage(requestError.data || requestError.message));
    } finally {
      setActionLoading(null);
    }
  };

  const verifySkill = async (skillId) => {
    setActionLoading(`skill-${skillId}`);
    setError("");
    setMessage("");
    try {
      const data = await apiRequest("/api/admin/verify-skill", {
        method: "POST",
        token,
        body: { worker_skill_id: skillId },
      });
      setMessage(data?.message || "Skill verified.");
      await loadOverview();
    } catch (requestError) {
      setError(extractErrorMessage(requestError.data || requestError.message));
    } finally {
      setActionLoading(null);
    }
  };

  const stats = overview?.stats || {};
  const pendingWorkers = overview?.pending_workers || [];
  const pendingSkills = overview?.pending_skills || [];

  return (
    <div className="space-y-5">
      <section className="rounded-2xl border border-slate-200/70 bg-white/90 p-5 shadow-panel">
        <PageHeader
          title="Admin Overview"
          subtitle="Snapshot of users, workers, requests, bookings and pending verifications."
          actions={
            <button
              type="button"
              onClick={loadOverview}
              disabled={loading}
              className="rounded-lg border border-slate-300 px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
            >
              {loading ? "Refreshing..." : "Refresh"}
            </button>
          }
        />

        {message ? (
          <p className="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
            {message}
          </p>
        ) : null}
        {error ? (
          <p className="mb-4 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
            {error}
          </p>
        ) : null}

        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {[
            ["Users", stats.users_total],
            ["Customers", stats.customers_total],
            ["Workers", stats.workers_total],
            ["Admins", stats.admins_total],
            ["Pending Worker Profiles", stats.pending_worker_profiles],
            ["Pending Worker Skills", stats.pending_worker_skills],
            ["Requests", stats.requests_total],
            ["Bookings", stats.bookings_total],
            ["Bookings In Progress", stats.bookings_in_progress],
          ].map(([label, value]) => (
            <article key={label} className="rounded-xl border border-slate-200 bg-white p-4">
              <p className="text-xs uppercase tracking-wide text-slate-500">{label}</p>
              <p className="mt-2 text-2xl font-semibold text-slate-900">{value ?? 0}</p>
            </article>
          ))}
        </div>
      </section>

      <section className="rounded-2xl border border-slate-200/70 bg-white/90 p-5 shadow-panel">
        <PageHeader
          title="Pending Worker Profiles"
          subtitle="Verify worker profile to allow availability updates and booking activity."
        />
        <div className="overflow-x-auto">
          <table className="min-w-full border-collapse text-sm">
            <thead>
              <tr className="border-b border-slate-200 text-left text-slate-500">
                <th className="px-2 py-2">Worker ID</th>
                <th className="px-2 py-2">User</th>
                <th className="px-2 py-2">Email</th>
                <th className="px-2 py-2">Action</th>
              </tr>
            </thead>
            <tbody>
              {pendingWorkers.map((worker) => (
                <tr key={worker.worker_profile_id} className="border-b border-slate-100">
                  <td className="px-2 py-2">#{worker.worker_profile_id}</td>
                  <td className="px-2 py-2">{worker.full_name}</td>
                  <td className="px-2 py-2">{worker.email}</td>
                  <td className="px-2 py-2">
                    <button
                      type="button"
                      onClick={() => verifyWorker(worker.worker_profile_id)}
                      disabled={actionLoading === `worker-${worker.worker_profile_id}`}
                      className="rounded-md bg-brand-600 px-2.5 py-1 text-xs font-semibold text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      {actionLoading === `worker-${worker.worker_profile_id}` ? "Verifying..." : "Verify Worker"}
                    </button>
                  </td>
                </tr>
              ))}
              {!pendingWorkers.length ? (
                <tr>
                  <td colSpan={4} className="px-2 py-4 text-center text-slate-500">
                    No pending workers.
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>

      <section className="rounded-2xl border border-slate-200/70 bg-white/90 p-5 shadow-panel">
        <PageHeader title="Pending Worker Skills" subtitle="Verify submitted worker skills." />
        <div className="overflow-x-auto">
          <table className="min-w-full border-collapse text-sm">
            <thead>
              <tr className="border-b border-slate-200 text-left text-slate-500">
                <th className="px-2 py-2">Skill ID</th>
                <th className="px-2 py-2">Worker</th>
                <th className="px-2 py-2">Category</th>
                <th className="px-2 py-2">Level</th>
                <th className="px-2 py-2">Price</th>
                <th className="px-2 py-2">Action</th>
              </tr>
            </thead>
            <tbody>
              {pendingSkills.map((skill) => (
                <tr key={skill.worker_skill_id} className="border-b border-slate-100">
                  <td className="px-2 py-2">#{skill.worker_skill_id}</td>
                  <td className="px-2 py-2">{skill.worker_full_name}</td>
                  <td className="px-2 py-2">{skill.category_name || `#${skill.category_id}`}</td>
                  <td className="px-2 py-2">{skill.experience_level}</td>
                  <td className="px-2 py-2">{skill.price_base}</td>
                  <td className="px-2 py-2">
                    <button
                      type="button"
                      onClick={() => verifySkill(skill.worker_skill_id)}
                      disabled={actionLoading === `skill-${skill.worker_skill_id}`}
                      className="rounded-md bg-brand-600 px-2.5 py-1 text-xs font-semibold text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      {actionLoading === `skill-${skill.worker_skill_id}` ? "Verifying..." : "Verify Skill"}
                    </button>
                  </td>
                </tr>
              ))}
              {!pendingSkills.length ? (
                <tr>
                  <td colSpan={6} className="px-2 py-4 text-center text-slate-500">
                    No pending skills.
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
