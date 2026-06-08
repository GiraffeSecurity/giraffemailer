import { GmSidebar } from '@/widgets/gmSidebar'

export default function GmLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <div className="gradient-mesh" />
      <div className="noise-overlay" />
      <div className="relative z-10 flex h-screen overflow-hidden">
        <GmSidebar />
        <main className="flex flex-1 flex-col overflow-hidden">
          {children}
        </main>
      </div>
    </>
  )
}
