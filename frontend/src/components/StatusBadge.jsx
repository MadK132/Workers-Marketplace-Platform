const styleMap = {
  pending: "bg-amber-100 text-amber-800",
  accepted: "bg-sky-100 text-sky-800",
  scheduled: "bg-indigo-100 text-indigo-800",
  in_progress: "bg-blue-100 text-blue-800",
  completed: "bg-emerald-100 text-emerald-800",
  cancelled: "bg-rose-100 text-rose-800",
  verified: "bg-emerald-100 text-emerald-800",
  active: "bg-emerald-100 text-emerald-800",
  inactive: "bg-slate-200 text-slate-700",
};

function toReadable(status) {
  return String(status || "")
    .replaceAll("_", " ")
    .replace(/\b\w/g, (m) => m.toUpperCase());
}

export default function StatusBadge({ status }) {
  const key = String(status || "").toLowerCase();
  const className = styleMap[key] || "bg-slate-100 text-slate-700";
  return (
    <span className={`inline-flex rounded-full px-2.5 py-1 text-xs font-semibold ${className}`}>
      {toReadable(status || "unknown")}
    </span>
  );
}
