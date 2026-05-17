import { Video } from "lucide-react";

type VideoProvider = "bilibili" | "youtube";

interface VideoEmbedProps {
  provider: VideoProvider;
  videoId: string;
  title?: string;
}

export function VideoEmbed({ provider, videoId, title }: VideoEmbedProps) {
  if (!videoId) {
    return <VideoComingSoon provider={provider} />;
  }

  const src =
    provider === "bilibili"
      ? `https://player.bilibili.com/player.html?bvid=${videoId}&autoplay=0&high_quality=1&danmaku=0`
      : `https://www.youtube.com/embed/${videoId}`;

  return (
    <div className="one-video-embed">
      <iframe
        allow="autoplay; encrypted-media; picture-in-picture; fullscreen"
        allowFullScreen
        loading="lazy"
        referrerPolicy="no-referrer"
        src={src}
        title={title ?? "Tutorial video"}
      />
    </div>
  );
}

function VideoComingSoon({ provider }: { provider: VideoProvider }) {
  const label = provider === "bilibili" ? "视频即将上线" : "Video coming soon";
  const hint =
    provider === "bilibili"
      ? "视频正在录制中，下方的文字版已经可以读。"
      : "Video is being recorded. The written walkthrough below is ready to read.";

  return (
    <div className="one-video-embed one-video-embed-placeholder" role="status">
      <div className="one-video-embed-placeholder-inner">
        <Video aria-hidden="true" />
        <strong>{label}</strong>
        <span>{hint}</span>
      </div>
    </div>
  );
}
