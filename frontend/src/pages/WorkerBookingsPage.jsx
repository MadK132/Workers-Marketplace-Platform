import { useEffect, useState } from "react";
import PageHeader from "../components/PageHeader";
import StatusBadge from "../components/StatusBadge";
import { useAuth } from "../context/AuthContext";
import { apiRequest, extractErrorMessage } from "../lib/api";

export default function WorkerBookingsPage() {
  const { token } = useAuth();
  const [bookings, setBookings] = useState([]);
  const [loading, setLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState(null);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  const loadBookings = async () => {
    setLoading(true);
    setError("");
    try {
      const data = await apiRequest("/api/bookings/my", { token });
      setBookings(data || []);
    } catch (requestError) {
      setError(extractErrorMessage(requestError.data || requestError.message));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadBookings();
  }, []);

  const updateStatus = async (bookingId, action) => {
    setActionLoading(`${action}-${bookingId}`);
    setMessage("");
    setError("");
    try {
      const data = await apiRequest(`/api/bookings/${bookingId}/${action}`, {
        method: "PATCH",
        token,
      });
      setMessage(data?.message || "Booking updated.");
      await loadBookings();
    } catch (requestError) {
      setError(extractErrorMessage(requestError.data || requestError.message));
    } finally {
      setActionLoading(null);
    }
  };

  return (
    <section className="rounded-2xl border border-slate-200/70 bg-white/90 p-5 shadow-panel">
      <PageHeader
        title="My Worker Bookings"
        subtitle="Start scheduled bookings and complete in-progress jobs."
        actions={
          <button
            type="button"
            onClick={loadBookings}
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

      <div className="grid gap-3">
        {bookings.map((booking) => (
          <article
            key={booking.booking_id}
            className="rounded-xl border border-slate-200 bg-white p-4"
          >
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h3 className="text-sm font-semibold text-slate-900">
                Booking #{booking.booking_id}
              </h3>
              <StatusBadge status={booking.status} />
            </div>
            <div className="mt-2 grid gap-1 text-sm text-slate-700 sm:grid-cols-2">
              <p>Request ID: #{booking.request_id}</p>
              <p>Customer: {booking.counterparty_name || "-"}</p>
              <p>Category: {booking.category_name || "-"}</p>
              <p>Description: {booking.request_description || "-"}</p>
            </div>
            <div className="mt-3 flex flex-wrap gap-2">
              {booking.status === "scheduled" ? (
                <button
                  type="button"
                  onClick={() => updateStatus(booking.booking_id, "start")}
                  disabled={actionLoading === `start-${booking.booking_id}`}
                  className="rounded-lg bg-brand-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {actionLoading === `start-${booking.booking_id}` ? "Starting..." : "Start"}
                </button>
              ) : null}
              {booking.status === "in_progress" ? (
                <button
                  type="button"
                  onClick={() => updateStatus(booking.booking_id, "complete")}
                  disabled={actionLoading === `complete-${booking.booking_id}`}
                  className="rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {actionLoading === `complete-${booking.booking_id}` ? "Completing..." : "Complete"}
                </button>
              ) : null}
            </div>
          </article>
        ))}
        {!bookings.length ? (
          <p className="rounded-lg border border-slate-200 bg-white px-3 py-4 text-sm text-slate-500">
            No bookings assigned yet.
          </p>
        ) : null}
      </div>
    </section>
  );
}
