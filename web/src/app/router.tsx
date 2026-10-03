import { createBrowserRouter, Navigate } from 'react-router'

import { AppShell } from '@/components/layout/AppShell'
import { RunsPage } from '@/features/runs/RunsPage'
import { WorkflowsPage } from '@/features/workflows/WorkflowsPage'

export const router = createBrowserRouter([
  {
    element: <AppShell />,
    children: [
      { index: true, element: <Navigate to="/workflows" replace /> },
      { path: 'workflows', element: <WorkflowsPage /> },
      { path: 'runs', element: <RunsPage /> },
      { path: '*', element: <Navigate to="/workflows" replace /> },
    ],
  },
])
