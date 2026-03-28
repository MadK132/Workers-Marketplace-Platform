import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { apiRequest, extractErrorMessage } from "../lib/api";

export default function RegisterPage() {
  const navigate = useNavigate();
  const auth = useAuth();

  const [form, setForm] = useState({
    full_name: "",
    email: "",
    phone: "",
    password: "",
    role: "customer",
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  useEffect(() => {
    if (auth.isAuthenticated) {
      navigate(auth.homePath, { replace: true });
    }
  }, [auth.homePath, auth.isAuthenticated, navigate]);

  const onSubmit = async (event) => {
    event.preventDefault();
    setError("");
    setSuccess("");
    setLoading(true);

    try {
      const payload = {
        ...form,
        phone: form.phone.trim() ? form.phone.trim() : null,
      };
      await apiRequest("/auth/register", { method: "POST", body: payload });
      setSuccess(
        "Account created. Check your email for verification before login.",
      );
    } catch (requestError) {
      const message = extractErrorMessage(requestError.data || requestError.message);
      if (requestError.status === 409 && message.toLowerCase().includes("email")) {
        setError(
          "This email is already used for another account role. One email can have only one role.",
        );
      } else {
        setError(message);
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="mx-auto flex min-h-screen max-w-lg items-center px-4 py-10">
      <div className="w-full rounded-2xl border border-slate-200/70 bg-white/90 p-6 shadow-panel backdrop-blur">
        <h1 className="text-2xl font-semibold text-slate-900">Create Account</h1>
        <p className="mt-1 text-sm text-slate-600">
          Register once with one role. Email cannot be reused for another role.
        </p>

        <form onSubmit={onSubmit} className="mt-6 space-y-4">
          <label className="block">
            <span className="mb-1 block text-sm font-medium text-slate-700">Full Name</span>
            <input
              type="text"
              required
              value={form.full_name}
              onChange={(e) => setForm((s) => ({ ...s, full_name: e.target.value }))}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
            />
          </label>

          <label className="block">
            <span className="mb-1 block text-sm font-medium text-slate-700">Email</span>
            <input
              type="email"
              required
              value={form.email}
              onChange={(e) => setForm((s) => ({ ...s, email: e.target.value }))}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
            />
          </label>

          <label className="block">
            <span className="mb-1 block text-sm font-medium text-slate-700">Phone (optional)</span>
            <input
              type="text"
              value={form.phone}
              onChange={(e) => setForm((s) => ({ ...s, phone: e.target.value }))}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
            />
          </label>

          <label className="block">
            <span className="mb-1 block text-sm font-medium text-slate-700">Password</span>
            <input
              type="password"
              required
              minLength={8}
              value={form.password}
              onChange={(e) => setForm((s) => ({ ...s, password: e.target.value }))}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
            />
          </label>

          <label className="block">
            <span className="mb-1 block text-sm font-medium text-slate-700">Role</span>
            <select
              value={form.role}
              onChange={(e) => setForm((s) => ({ ...s, role: e.target.value }))}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
            >
              <option value="customer">Customer</option>
              <option value="worker">Worker</option>
            </select>
          </label>

          {error ? (
            <p className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
              {error}
            </p>
          ) : null}
          {success ? (
            <p className="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
              {success}
            </p>
          ) : null}

          <button
            type="submit"
            disabled={loading}
            className="w-full rounded-lg bg-brand-600 px-4 py-2.5 text-sm font-semibold text-white transition hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {loading ? "Registering..." : "Register"}
          </button>
        </form>

        <p className="mt-4 text-sm text-slate-600">
          Already have an account?{" "}
          <Link className="font-semibold text-brand-700 hover:text-brand-800" to="/login">
            Sign In
          </Link>
        </p>
      </div>
    </div>
  );
}
