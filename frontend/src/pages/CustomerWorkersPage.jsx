import { useEffect, useState } from "react";
import PageHeader from "../components/PageHeader";
import { useAuth } from "../context/AuthContext";
import { apiRequest, extractErrorMessage } from "../lib/api";

export default function CustomerWorkersPage() {
  const { token } = useAuth();
  const [categories, setCategories] = useState([]);
  const [selectedCategory, setSelectedCategory] = useState("");
  const [workers, setWorkers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [bookingForm, setBookingForm] = useState({
    request_id: "",
    worker_profile_id: "",
  });
  const [bookingMessage, setBookingMessage] = useState("");

  useEffect(() => {
    let isMounted = true;
    async function bootstrap() {
      try {
        const data = await apiRequest("/api/categories", { token });
        if (!isMounted) return;
        setCategories(data || []);
        if (data?.length) {
          setSelectedCategory(String(data[0].category_id));
        }
      } catch (requestError) {
        if (!isMounted) return;
        setError(extractErrorMessage(requestError.data || requestError.message));
      }
    }
    bootstrap();
    return () => {
      isMounted = false;
    };
  }, [token]);

  const findWorkers = async () => {
    if (!selectedCategory) return;
    setLoading(true);
    setError("");
    try {
      const data = await apiRequest(`/api/workers?category_id=${selectedCategory}`, { token });
      setWorkers(data || []);
    } catch (requestError) {
      setError(extractErrorMessage(requestError.data || requestError.message));
    } finally {
      setLoading(false);
    }
  };

  const createBooking = async (event) => {
    event.preventDefault();
    setBookingMessage("");
    setError("");
    try {
      const payload = {
        request_id: Number(bookingForm.request_id),
        worker_profile_id: Number(bookingForm.worker_profile_id),
      };
      const data = await apiRequest("/api/bookings", {
        method: "POST",
        token,
        body: payload,
      });
      setBookingMessage(data?.message || "Booking created.");
    } catch (requestError) {
      setError(extractErrorMessage(requestError.data || requestError.message));
    }
  };

  return (
    <div className="space-y-5">
      <section className="rounded-2xl border border-slate-200/70 bg-white/90 p-5 shadow-panel">
        <PageHeader
          title="Find Workers"
          subtitle="Select category and load available verified workers."
          actions={
            <button
              type="button"
              onClick={findWorkers}
              disabled={loading || !selectedCategory}
              className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {loading ? "Loading..." : "Load Workers"}
            </button>
          }
        />

        <label className="block max-w-sm">
          <span className="mb-1 block text-sm font-medium text-slate-700">Category</span>
          <select
            value={selectedCategory}
            onChange={(e) => setSelectedCategory(e.target.value)}
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

        {error ? (
          <p className="mt-4 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
            {error}
          </p>
        ) : null}

        <div className="mt-4 overflow-x-auto">
          <table className="min-w-full border-collapse text-sm">
            <thead>
              <tr className="border-b border-slate-200 text-left text-slate-500">
                <th className="px-2 py-2">Worker ID</th>
                <th className="px-2 py-2">Name</th>
                <th className="px-2 py-2">Category</th>
                <th className="px-2 py-2">Experience</th>
                <th className="px-2 py-2">Price</th>
                <th className="px-2 py-2">Action</th>
              </tr>
            </thead>
            <tbody>
              {workers.map((worker) => (
                <tr key={`${worker.worker_id}-${worker.full_name}`} className="border-b border-slate-100">
                  <td className="px-2 py-2">#{worker.worker_id}</td>
                  <td className="px-2 py-2">{worker.full_name}</td>
                  <td className="px-2 py-2">{worker.category_name}</td>
                  <td className="px-2 py-2">{worker.experience_level}</td>
                  <td className="px-2 py-2">{worker.price}</td>
                  <td className="px-2 py-2">
                    <button
                      type="button"
                      onClick={() =>
                        setBookingForm((s) => ({
                          ...s,
                          worker_profile_id: String(worker.worker_id),
                        }))
                      }
                      className="rounded-md border border-slate-300 px-2 py-1 text-xs font-medium text-slate-700 hover:bg-slate-50"
                    >
                      Use For Booking
                    </button>
                  </td>
                </tr>
              ))}
              {!workers.length ? (
                <tr>
                  <td colSpan={6} className="px-2 py-4 text-center text-slate-500">
                    No workers loaded.
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>

      <section className="rounded-2xl border border-slate-200/70 bg-white/90 p-5 shadow-panel">
        <PageHeader
          title="Create Booking"
          subtitle="Use your request ID and selected worker profile ID."
        />
        <form onSubmit={createBooking} className="grid gap-3 sm:grid-cols-3">
          <label className="block">
            <span className="mb-1 block text-sm font-medium text-slate-700">Request ID</span>
            <input
              type="number"
              min={1}
              required
              value={bookingForm.request_id}
              onChange={(e) => setBookingForm((s) => ({ ...s, request_id: e.target.value }))}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
            />
          </label>
          <label className="block">
            <span className="mb-1 block text-sm font-medium text-slate-700">Worker Profile ID</span>
            <input
              type="number"
              min={1}
              required
              value={bookingForm.worker_profile_id}
              onChange={(e) => setBookingForm((s) => ({ ...s, worker_profile_id: e.target.value }))}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
            />
          </label>
          <div className="flex items-end">
            <button
              type="submit"
              className="w-full rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700"
            >
              Create Booking
            </button>
          </div>
        </form>
        {bookingMessage ? (
          <p className="mt-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
            {bookingMessage}
          </p>
        ) : null}
      </section>
    </div>
  );
}
