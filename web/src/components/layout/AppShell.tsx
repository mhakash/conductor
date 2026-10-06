import { Outlet, useMatch } from 'react-router'

import { Sidebar } from './Sidebar'

export function AppShell() {
  // The editor needs the horizontal space for its palette, canvas and inspector.
  const workflow = useMatch('/workflows/:id')
  const scratch = useMatch('/scratch')
  const inEditor = workflow !== null || scratch !== null

  return (
    <div className="flex h-full">
      <Sidebar rail={inEditor} />
      <main className="flex min-w-0 flex-1 flex-col">
        <Outlet />
      </main>
    </div>
  )
}
