import { GmChangePassword } from '@/widgets/gmChangePassword'

export default function SettingsPage() {
  return (
    <div className="h-full overflow-auto p-8 lg:p-10 animate-slide-up">
      <div className="mx-auto max-w-2xl space-y-6">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-foreground">Settings</h1>
          <p className="mt-1 text-[13px] text-muted-foreground">
            Manage your account. Server options are set in the process config file.
          </p>
        </div>

        <GmChangePassword />

        <div className="gm-card">
          <h2 className="text-[15px] font-semibold text-foreground">Server configuration</h2>
          <p className="mt-2 text-[13px] text-muted-foreground">
            Edit the config file and restart GiraffeMail to change server settings.
            Source installs typically use{' '}
            <code className="rounded-md bg-secondary px-1.5 py-0.5 text-[12px] text-primary">config.yaml</code>.
            Docker builds copy{' '}
            <code className="rounded-md bg-secondary px-1.5 py-0.5 text-[12px] text-primary">config.docker.yaml</code> to{' '}
            <code className="rounded-md bg-secondary px-1.5 py-0.5 text-[12px] text-primary">/etc/giraffemail/config.yaml</code>{' '}
            inside the image — do not rename the Docker source file on the host.
          </p>
          <p className="mt-4 mb-2 text-[12px] text-muted-foreground">Key fields</p>
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
