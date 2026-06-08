export default function SettingsPage() {
  return (
    <div className="h-full overflow-auto p-8 lg:p-10 animate-slide-up">
      <div className="mx-auto max-w-2xl">
        <h1 className="text-xl font-bold tracking-tight text-foreground">Settings</h1>
        <p className="mt-1 text-[13px] text-muted-foreground">
          Edit <code className="rounded-md bg-secondary px-1.5 py-0.5 text-[12px] text-primary">config.yaml</code> and restart the server to change settings.
        </p>
        <div className="gm-card mt-5">
          <p className="mb-2 text-[12px] text-muted-foreground">config.yaml — key fields</p>
          <pre className="overflow-x-auto text-[12px] leading-relaxed text-foreground/80">{`app:
  env: dev | production
  port: 9191
  secret_key: <64-char hex>

storage:
  data_dir: ./data
  encrypt_blobs: false

archive:
  worker_count: 4
  batch_size_bytes: 8388608

smtp:
  host: ""
  port: 587
  username: ""
  password: ""
  from: ""
`}</pre>
        </div>
      </div>
    </div>
  )
}
