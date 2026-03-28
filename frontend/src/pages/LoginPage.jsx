import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth, resolveHomePath } from "../context/AuthContext";
import { apiRequest, extractErrorMessage } from "../lib/api";
import { parseJwt } from "../lib/jwt";

export default function LoginPage() {
  const navigate = useNavigate();
  const auth = useAuth();

  const [form, setForm] = useState({
    email: "",
    password: "",
  });
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (auth.isAuthenticated) {
      navigate(auth.homePath, { replace: true });
    }
  }, [auth.homePath, auth.isAuthenticated, navigate]);

  const onSubmit = async (event) => {
    event.preventDefault();
    setError("");
    setLoading(true);

    try {
      const data = await apiRequest("/auth/login", {
        method: "POST",
        body: form,
      });
      const payload = parseJwt(data.access_token);
      auth.login(data);
      navigate(resolveHomePath(payload?.role), { replace: true });
    } catch (requestError) {
      setError(extractErrorMessage(requestError.data || requestError.message));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="mx-auto flex min-h-screen max-w-md items-center px-4 py-10">
      <div className="w-full rounded-2xl border border-slate-200/70 bg-white/90 p-6 shadow-panel backdrop-blur">
        <h1 className="text-2xl font-semibold text-slate-900">Sign In</h1>
        <p className="mt-1 text-sm text-slate-600">Use your marketplace account credentials.</p>

        <form onSubmit={onSubmit} className="mt-6 space-y-4">
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

          {error ? (
            <p className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
              {error}
            </p>
          ) : null}

          <button
            type="submit"
            disabled={loading}
            className="w-full rounded-lg bg-brand-600 px-4 py-2.5 text-sm font-semibold text-white transition hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {loading ? "Signing in..." : "Sign In"}
          </button>
        </form>

        <p className="mt-4 text-sm text-slate-600">
          No account yet?{" "}
          <Link className="font-semibold text-brand-700 hover:text-brand-800" to="/register">
            Register
          </Link>
        </p>
      </div>
    </div>
  );
}
