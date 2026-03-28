import { NavLink, Outlet } from "react-router-dom";
import { useAuth } from "../context/AuthContext";

const navConfig = {
  customer: [
    { to: "/customer/profile", label: "Profile" },
    { to: "/customer/workers", label: "Workers" },
    { to: "/customer/requests", label: "Requests" },
    { to: "/customer/bookings", label: "Bookings" },
  ],
  worker: [
    { to: "/worker/profile", label: "Profile" },
    { to: "/worker/skills", label: "Skills" },
    { to: "/worker/bookings", label: "Bookings" },
  ],
  admin: [{ to: "/admin/overview", label: "Overview" }],
};

function roleLabel(role) {
  if (role === "admin") return "Admin";
  if (role === "worker") return "Worker";
  return "Customer";
}

export default function AppLayout() {
  const { role, logout } = useAuth();
  const navItems = navConfig[role] || [];

  return (
    <div className="mx-auto flex min-h-screen max-w-7xl flex-col px-4 py-6 sm:px-6 lg:px-8">
      <header className="mb-6 rounded-2xl border border-slate-200/60 bg-white/85 p-4 shadow-panel backdrop-blur">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="text-xs uppercase tracking-wide text-slate-500">Workers Marketplace</p>
            
          </div>
          <div className="flex items-center gap-2">
            <span className="rounded-full bg-brand-100 px-3 py-1 text-xs font-semibold uppercase text-brand-800">
              {roleLabel(role)}
            </span>
            <button
              type="button"
              onClick={logout}
              className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 transition hover:border-slate-400 hover:bg-slate-50"
            >
              Logout
            </button>
          </div>
        </div>
        <nav className="mt-4 flex flex-wrap gap-2">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                `rounded-lg px-3 py-2 text-sm font-medium transition ${
                  isActive
                    ? "bg-brand-600 text-white shadow"
                    : "border border-slate-200 text-slate-700 hover:border-brand-200 hover:bg-brand-50 hover:text-brand-700"
                }`
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
      </header>
      <main className="flex-1 pb-8">
        <Outlet />
      </main>
    </div>
  );
}
