import { Toaster as Sonner, type ToasterProps } from "sonner";

// 简化版：不接 next-themes，由父层自行控制 className=dark
// 需要夜间模式时把 theme 传进来或从全局 store 读
const Toaster = (props: ToasterProps) => {
  return (
    <Sonner
      className="toaster group"
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
        } as React.CSSProperties
      }
      {...props}
    />
  );
};

export { Toaster };
