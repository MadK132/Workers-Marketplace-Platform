import { useEffect, useState } from "react";
import PageHeader from "../components/PageHeader";
import StatusBadge from "../components/StatusBadge";
import { useAuth } from "../context/AuthContext";
import { apiRequest, extractErrorMessage } from "../lib/api";

export default function CustomerRequestsPage() {
  const { token } = useAuth();

  const [categories, setCategories] = useState([]);
  const [form, setForm] = useState({
    category_id: "",
    description: "",
  });
  const [requests, setRequests] = useState([]);
  const [loadingRequests, setLoadingRequests] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  const loadRequests = async () => {
    setLoadingRequests(true);
    try {
      const data = await apiRequest("/api/requests/my", { token });
      setRequests(data || []);
    } catch (requestError) {
      setError(extractErrorMessage(requestError.data || requestError.message));
    } finally {
      setLoadingRequests(false);
    }
  };

  useEffect(() => {
    let isMounted = true;
    async function bootstrap() {
      try {
        const categoryData = await apiRequest("/api/categories", { token });
        if (!isMounted) return;
        setCategories(categoryData || []);
        if (categoryData?.length) {
          setForm((s) => ({ ...s, category_id: String(categoryData[0].category_id) }));
        }
      } catch (requestError) {
        if (!isMounted) return;
        setError(extractErrorMessage(requestError.data || requestError.message));
      }
      if (isMounted) {
        await loadRequests();
      }
    }
    bootstrap();
    return () => {
      isMounted = false;
    };
  }, [token]);

  const createRequest = async (event) => {
    event.preventDefault();
    if (!form.category_id) {
      setError("No service categories found. Ask admin to add or seed categories.");
      return;
    }
    setError("");
    setMessage("");
    setSubmitting(true);

    try {
      const data = await apiRequest("/api/requests", {
        method: "POST",
        token,
        body: {
          category_id: Number(form.category_id),
          description: form.description,
        },
      });
      setMessage(data?.message || "Request created.");
      setForm((s) => ({ ...s, description: "" }));
      await loadRequests();
    } catch (requestError) {
      setError(extractErrorMessage(requestError.data || requestError.message));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="space-y-5">
      <section className="rounded-2xl border border-slate-200/70 bg-white/90 p-5 shadow-panel">
        <PageHeader
          title="Create Service Request"
          subtitle="Create request first, then use request ID in booking screen."
        />
        <form onSubmit={createRequest} className="grid gap-3 sm:grid-cols-3">
          <label className="block">
            <span className="mb-1 block text-sm font-medium text-slate-700">Category</span>
            <select
              value={form.category_id}
              onChange={(e) => setForm((s) => ({ ...s, category_id: e.target.value }))}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
            >
              {!categories.length ? <option value="">No categories yet</option> : null}
              {categories.map((category) => (
                <option key={category.category_id} value={String(category.category_id)}>
                  {category.name} (#{category.category_id})
                </option>
              ))}
            </select>
          </label>

          <label className="block sm:col-span-2">
            <span className="mb-1 block text-sm font-medium text-slate-700">Description</span>
            <input
              type="text"
              required
              value={form.description}
              onChange={(e) => setForm((s) => ({ ...s, description: e.target.value }))}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
            />
          </label>

          <div className="sm:col-span-3">
            <button
              type="submit"
              disabled={submitting || !form.category_id}
              className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {submitting ? "Submitting..." : "Submit Request"}
            </button>
          </div>
        </form>

        {!form.category_id ? (
          <p className="mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
            No categories in database yet. Seed `service_categories` first.
          </p>
        ) : null}

        {message ? (
          <p className="mt-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
            {message}
          </p>
        ) : null}
        {error ? (
          <p className="mt-3 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
            {error}
          </p>
        ) : null}
      </section>

      <section className="rounded-2xl border border-slate-200/70 bg-white/90 p-5 shadow-panel">
        <PageHeader
          title="My Requests"
          subtitle="IDs from this list are used when creating booking."
          actions={
            <button
              type="button"
              onClick={loadRequests}
              disabled={loadingRequests}
              className="rounded-lg border border-slate-300 px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
            >
              {loadingRequests ? "Refreshing..." : "Refresh"}
            </button>
          }
        />

        <div className="overflow-x-auto">
          <table className="min-w-full border-collapse text-sm">
            <thead>
              <tr className="border-b border-slate-200 text-left text-slate-500">
                <th className="px-2 py-2">Request ID</th>
                <th className="px-2 py-2">Category</th>
                <th className="px-2 py-2">Description</th>
                <th className="px-2 py-2">Status</th>
                <th className="px-2 py-2">Created</th>
              </tr>
            </thead>
            <tbody>
              {requests.map((request) => (
                <tr key={request.request_id} className="border-b border-slate-100">
                  <td className="px-2 py-2 font-medium text-slate-800">#{request.request_id}</td>
                  <td className="px-2 py-2">
                    {request.category_name || "Category"} (#{request.category_id})
                  </td>
                  <td className="px-2 py-2">{request.description}</td>
                  <td className="px-2 py-2">
                    <StatusBadge status={request.status} />
                  </td>
                  <td className="px-2 py-2 text-slate-600">
                    {new Date(request.created_at).toLocaleString()}
                  </td>
                </tr>
              ))}
              {!requests.length ? (
                <tr>
                  <td colSpan={5} className="px-2 py-4 text-center text-slate-500">
                    No requests yet.
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
