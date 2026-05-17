import { redirect } from "next/navigation";

export const metadata = {
  title: "Template Builder | One CLI",
  robots: {
    index: false,
    follow: true,
  },
};

export default function LegacyTemplatesRoute() {
  redirect("/zh/templates/");
}
