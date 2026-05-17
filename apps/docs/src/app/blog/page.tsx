import { redirect } from "next/navigation";

export const metadata = {
  title: "Blog | One CLI",
  robots: {
    index: false,
    follow: true,
  },
};

export default function LegacyBlogRoute() {
  redirect("/zh/blog/");
}
