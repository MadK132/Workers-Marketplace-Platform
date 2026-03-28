import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { apiRequest, extractErrorMessage } from "../lib/api";

export default function VerifyEmailPage() {
  const [search] = useSearchParams();
  const token = search.get("token");
  const [status, setStatus] = useState("loading");
  const [message, setMessage] = useState("");

  useEffect(() => {
    let isMounted = true;

    async function run() {
      if (!token) {
        setStatus("error");
        setMessage("Missing token.");
        return;
      }

      try {
        const data = await apiRequest(`/auth/verify?token=${encodeURIComponent(token)}`);
        if (!isMounted) return;
        setStatus("success");
        setMessage(data?.message || "Email verified.");
      } catch (error) {
        if (!isMounted) return;
        setStatus("error");
        setMessage(extractErrorMessage(error.data || error.message));
      }
    }

    run();
    return () => {
      isMounted = false;
    };
  }, [token]);

  return (
    <div className="mx-auto flex min-h-screen max-w-md items-center px-4 py-10">
      <div className="w-full rounded-2xl border border-slate-200/70 bg-white/90 p-6 shadow-panel">
        <h1 className="text-2xl font-semibold text-slate-900">Email Verification</h1>
        <p className="mt-3 text-sm text-slate-700">
          {status === "loading" ? "Verifying..." : message}
        </p>
        <Link
          className="mt-5 inline-flex rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700"
          to="/login"
        >
          Go To Login
        </Link>
      </div>
    </div>
  );
}
