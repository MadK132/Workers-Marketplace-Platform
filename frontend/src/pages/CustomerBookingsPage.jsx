import { useEffect, useState } from "react";
import PageHeader from "../components/PageHeader";
import StatusBadge from "../components/StatusBadge";
import { useAuth } from "../context/AuthContext";
import { apiRequest, extractErrorMessage } from "../lib/api";

export default function CustomerBookingsPage() {
  const { token } = useAuth();
  const [bookings, setBookings] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

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

  return (
    <section className="rounded-2xl border border-slate-200/70 bg-white/90 p-5 shadow-panel">
      <PageHeader
        title="My Bookings"
        subtitle="Bookings are loaded directly from backend."
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
              <p>Worker ID: #{booking.worker_profile_id}</p>
              <p>Category: {booking.category_name || "Unknown"}</p>
              <p>Description: {booking.request_description || "-"}</p>
              <p>
                Counterparty: {booking.counterparty_name || "-"} ({booking.counterparty_role})
              </p>
              <p>Final Price: {booking.final_price || "-"}</p>
            </div>
          </article>
        ))}
        {!bookings.length ? (
          <p className="rounded-lg border border-slate-200 bg-white px-3 py-4 text-sm text-slate-500">
            No bookings yet.
          </p>
        ) : null}
      </div>
    </section>
  );
}
