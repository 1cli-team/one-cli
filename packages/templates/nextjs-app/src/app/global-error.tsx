"use client";

import NextError from "next/error";
import { useEffect } from "react";

export default function GlobalError({ error }: { error: Error & { digest?: string } }) {
	useEffect(() => {
		console.error("Global error captured", error);
	}, [error]);

	return (
		<html lang="en">
			<body>
				<NextError statusCode={0} />
			</body>
		</html>
	);
}
