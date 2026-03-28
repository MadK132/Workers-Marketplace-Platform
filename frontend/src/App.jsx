import { Navigate, Route, Routes } from "react-router-dom";
import AppLayout from "./components/AppLayout";
import ProtectedRoute from "./components/ProtectedRoute";
import { useAuth } from "./context/AuthContext";
import AdminOverviewPage from "./pages/AdminOverviewPage";
import CustomerBookingsPage from "./pages/CustomerBookingsPage";
import CustomerProfilePage from "./pages/CustomerProfilePage";
import CustomerRequestsPage from "./pages/CustomerRequestsPage";
import CustomerWorkersPage from "./pages/CustomerWorkersPage";
import LoginPage from "./pages/LoginPage";
import RegisterPage from "./pages/RegisterPage";
import VerifyEmailPage from "./pages/VerifyEmailPage";
import WorkerBookingsPage from "./pages/WorkerBookingsPage";
import WorkerProfilePage from "./pages/WorkerProfilePage";
import WorkerSkillsPage from "./pages/WorkerSkillsPage";

function RootRedirect() {
  const { isAuthenticated, homePath } = useAuth();
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  return <Navigate to={homePath} replace />;
}

function NotFound() {
  return (
    <div className="mx-auto max-w-xl rounded-2xl border border-slate-200 bg-white p-6 text-center shadow-panel">
      <h1 className="text-2xl font-semibold text-slate-900">Page Not Found</h1>
      <p className="mt-2 text-sm text-slate-600">The route does not exist in current MVP.</p>
    </div>
  );
}

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<RootRedirect />} />
      <Route path="/login" element={<LoginPage />} />
      <Route path="/register" element={<RegisterPage />} />
      <Route path="/verify-email" element={<VerifyEmailPage />} />

      <Route element={<ProtectedRoute />}>
        <Route element={<AppLayout />}>
          <Route
            path="/customer/profile"
            element={
              <ProtectedRoute allowRoles={["customer"]}>
                <CustomerProfilePage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/customer/workers"
            element={
              <ProtectedRoute allowRoles={["customer"]}>
                <CustomerWorkersPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/customer/requests"
            element={
              <ProtectedRoute allowRoles={["customer"]}>
                <CustomerRequestsPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/customer/bookings"
            element={
              <ProtectedRoute allowRoles={["customer"]}>
                <CustomerBookingsPage />
              </ProtectedRoute>
            }
          />

          <Route
            path="/worker/profile"
            element={
              <ProtectedRoute allowRoles={["worker"]}>
                <WorkerProfilePage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/worker/skills"
            element={
              <ProtectedRoute allowRoles={["worker"]}>
                <WorkerSkillsPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/worker/bookings"
            element={
              <ProtectedRoute allowRoles={["worker"]}>
                <WorkerBookingsPage />
              </ProtectedRoute>
            }
          />

          <Route
            path="/admin/overview"
            element={
              <ProtectedRoute allowRoles={["admin"]}>
                <AdminOverviewPage />
              </ProtectedRoute>
            }
          />

          <Route path="*" element={<NotFound />} />
        </Route>
      </Route>
    </Routes>
  );
}
