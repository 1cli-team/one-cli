import { generateHomeMetadata, LocalizedHomePage } from "../(home)/home-page";

export function generateMetadata() {
	return generateHomeMetadata("zh");
}

export default function Page() {
	return <LocalizedHomePage lang="zh" />;
}
