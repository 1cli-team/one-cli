import Image, { type ImageProps } from "next/image";

type TemplateCoverImageProps = Omit<
  ImageProps,
  "alt" | "fill" | "height" | "src" | "width"
> & {
  src: string;
  alt?: string;
  sizes: string;
};

export function TemplateCoverImage({
  src,
  sizes,
  alt = "",
  ...props
}: TemplateCoverImageProps) {
  return (
    <Image
      {...props}
      alt={alt}
      fill
      sizes={sizes}
      src={src}
    />
  );
}
