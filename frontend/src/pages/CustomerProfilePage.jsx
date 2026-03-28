import { useState } from "react";
import PageHeader from "../components/PageHeader";
import { useAuth } from "../context/AuthContext";
import { apiRequest, extractErrorMessage } from "../lib/api";

export default function CustomerProfilePage() {
  const { token } = useAuth();
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const createProfile = async () => {
    setLoading(true);
    setError("");
    setMessage("");
    try {
      const data = await apiRequest("/api/customer/profile", {
        method: "POST",
        token,
      });
      setMessage(data?.message || "Customer profile created.");
    } catch (requestError) {
      const text = extractErrorMessage(requestError.data || requestError.message);
      if (text.toLowerCase().includes("duplicate")) {
        setMessage("Customer profile already exists.");
      } else {
        setError(text);
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <section className="rounded-2xl border border-slate-200/70 bg-white/90 p-5 shadow-panel">
      <PageHeader
        title="Customer Profile"
        subtitle="Create your customer profile once before creating requests or bookings."
      />
      <button
        type="button"
        onClick={createProfile}
        disabled={loading}
        className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-60"
      >
        {loading ? "Creating..." : "Create Customer Profile"}
      </button>

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
