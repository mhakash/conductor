import { Outlet } from 'react-router'

import { Sidebar } from './Sidebar'

export function AppShell() {
  return (
    <div className="flex h-full">
      <Sidebar />
      <main className="flex min-w-0 flex-1 flex-col">
        <Outlet />
      </main>
    </div>
  )
}
