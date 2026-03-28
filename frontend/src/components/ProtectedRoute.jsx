import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "../context/AuthContext";

export default function ProtectedRoute({ allowRoles, children }) {
  const { isAuthenticated, role, homePath } = useAuth();

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  if (allowRoles && allowRoles.length > 0 && !allowRoles.includes(role)) {
    return <Navigate to={homePath} replace />;
  }

  if (children) {
    return children;
  }

  return <Outlet />;
}
