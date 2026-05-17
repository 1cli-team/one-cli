import { defaultLocale } from "@/i18n";
import {
  generateHomeMetadata,
  LocalizedHomePage,
} from "./home-page";

export function generateMetadata() {
  return generateHomeMetadata(defaultLocale);
}

export default function HomePage() {
  return <LocalizedHomePage lang={defaultLocale} />;
}
