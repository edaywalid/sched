import { Route, Routes } from "react-router";
import { LandingPage } from "./routes/Landing";
import { DashboardLayout } from "./routes/DashboardLayout";
import { WorkflowsPage } from "./routes/Workflows";
import { WorkflowDetailPage } from "./routes/WorkflowDetail";
import { NotFoundPage } from "./routes/NotFound";

export function App() {
  return (
    <Routes>
      <Route index element={<LandingPage />} />
      <Route path="app" element={<DashboardLayout />}>
        <Route index element={<WorkflowsPage />} />
        <Route path="workflows/:id" element={<WorkflowDetailPage />} />
      </Route>
      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  );
}
