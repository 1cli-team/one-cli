import useSWR from "swr";
import type { AppInfo } from "@one-template-electron/preload/channels";

export default function HomePage() {
  const { data, error } = useSWR<AppInfo>(
    window.electron ? "app.info" : null,
    () => window.electron!.getAppInfo(),
  );
  return (
    <section className="space-y-4">
      <h1 className="text-3xl font-semibold">欢迎 / Welcome</h1>
      <p className="text-muted-foreground">
        从这里开始构建桌面应用。Start building your desktop application here.
      </p>
      {data && (
        <p>
          {data.name} · {data.version} · {data.platform}
        </p>
      )}
      {error && (
        <p role="alert">无法读取应用信息 / Could not load app information</p>
      )}
    </section>
  );
}
