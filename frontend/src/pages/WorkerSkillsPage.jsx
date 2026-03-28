import { useEffect, useState } from "react";
import PageHeader from "../components/PageHeader";
import { useAuth } from "../context/AuthContext";
import { apiRequest, extractErrorMessage } from "../lib/api";

export default function WorkerSkillsPage() {
  const { token } = useAuth();
  const [categories, setCategories] = useState([]);
  const [form, setForm] = useState({
    category_id: "",
    experience_level: "junior",
    price_base: "100",
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  useEffect(() => {
    let isMounted = true;
    async function bootstrap() {
      try {
        const data = await apiRequest("/api/categories", { token });
        if (!isMounted) return;
        setCategories(data || []);
        if (data?.length) {
          setForm((s) => ({ ...s, category_id: String(data[0].category_id) }));
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

  const addSkill = async (event) => {
    event.preventDefault();
    if (!form.category_id) {
      setError("No service categories found. Ask admin to add or seed categories.");
      return;
    }
    setLoading(true);
    setError("");
    setMessage("");
    try {
      const data = await apiRequest("/api/worker/skills", {
        method: "POST",
        token,
        body: {
          category_id: Number(form.category_id),
          experience_level: form.experience_level,
          price_base: Number(form.price_base),
        },
      });
      setMessage(data?.message || "Skill added. Admin verification required.");
    } catch (requestError) {
      setError(extractErrorMessage(requestError.data || requestError.message));
    } finally {
      setLoading(false);
    }
  };

  return (
    <section className="rounded-2xl border border-slate-200/70 bg-white/90 p-5 shadow-panel">
      <PageHeader
        title="Worker Skills"
        subtitle="Submit skill + level + base price. Admin can verify from admin panel."
      />

      <form onSubmit={addSkill} className="grid gap-3 sm:grid-cols-3">
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

        <label className="block">
          <span className="mb-1 block text-sm font-medium text-slate-700">Experience Level</span>
          <select
            value={form.experience_level}
            onChange={(e) => setForm((s) => ({ ...s, experience_level: e.target.value }))}
            className="w-full rounded-lg border border-slate-300 px-3 py-2 outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
          >
            <option value="junior">Junior</option>
            <option value="middle">Middle</option>
            <option value="senior">Senior</option>
          </select>
        </label>

        <label className="block">
          <span className="mb-1 block text-sm font-medium text-slate-700">Base Price</span>
          <input
            type="number"
            min={1}
            required
            value={form.price_base}
            onChange={(e) => setForm((s) => ({ ...s, price_base: e.target.value }))}
            className="w-full rounded-lg border border-slate-300 px-3 py-2 outline-none transition focus:border-brand-500 focus:ring-2 focus:ring-brand-100"
          />
        </label>

        <div className="sm:col-span-3">
          <button
            type="submit"
            disabled={loading || !form.category_id}
            className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-semibold text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {loading ? "Submitting..." : "Submit Skill"}
          </button>
        </div>
      </form>

      {!form.category_id ? (
        <p className="mt-4 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
          No categories in database yet. Seed `service_categories` first.
        </p>
      ) : null}

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
