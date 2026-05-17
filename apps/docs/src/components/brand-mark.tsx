type BrandMarkProps = {
  className?: string;
  variant?: "dark" | "light";
};

export function BrandMark({ className = "", variant = "light" }: BrandMarkProps) {
  const isDark = variant === "dark";
  const rootClass = ["inline-flex items-center", className].filter(Boolean).join(" ");
  const src = isDark ? "/brand/logo-inverted.svg" : "/brand/logo.svg";

  return (
    <span className={rootClass}>
      <img src={src} alt="" className="h-8 w-auto" />
    </span>
  );
}
