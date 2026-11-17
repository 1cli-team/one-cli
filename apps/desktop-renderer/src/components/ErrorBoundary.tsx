import { AlertTriangle, Home, RotateCcw } from "lucide-react";
import React from "react";
import { useTranslation } from "react-i18next";
import { LanguageSwitcher } from "@/components/LanguageSwitcher";
import { Button } from "@/components/ui/button";

interface ErrorBoundaryState {
	hasError: boolean;
	error?: Error;
	errorId?: string;
}

interface ErrorBoundaryProps {
	children: React.ReactNode;
	fallback?: React.ComponentType<{ error?: Error; resetError: () => void }>;
	onError?: (error: Error, errorInfo: React.ErrorInfo) => void;
}

class ErrorBoundaryClass extends React.Component<ErrorBoundaryProps, ErrorBoundaryState> {
	constructor(props: ErrorBoundaryProps) {
		super(props);
		this.state = { hasError: false };
	}

	static getDerivedStateFromError(error: Error): ErrorBoundaryState {
		// 生成错误 ID
		const errorId = Math.random().toString(36).substring(2, 15);
		return {
			hasError: true,
			error,
			errorId,
		};
	}

	componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
		// 调用自定义错误处理
		this.props.onError?.(error, errorInfo);

		// 开发环境下打印详细错误信息
		console.group("🚨 ErrorBoundary caught an error");
		console.error("Error:", error);
		console.error("Component Stack:", errorInfo.componentStack);
		console.groupEnd();
	}

	handleReset = () => {
		this.setState({ hasError: false, error: undefined, errorId: undefined });
	};

	render() {
		if (this.state.hasError) {
			// 如果提供了自定义 fallback 组件
			if (this.props.fallback) {
				const FallbackComponent = this.props.fallback;
				return <FallbackComponent error={this.state.error} resetError={this.handleReset} />;
			}

			// 默认错误 UI
			return (
				<DefaultErrorFallback
					error={this.state.error}
					resetError={this.handleReset}
					errorId={this.state.errorId}
				/>
			);
		}

		return this.props.children;
	}
}

// 默认错误回退组件
const DefaultErrorFallback: React.FC<{
	error?: Error;
	resetError: () => void;
	errorId?: string;
}> = ({ error, resetError, errorId }) => {
	const isDev = !import.meta.env.PROD;
	const { t } = useTranslation();

	return (
		<div className="window-fallback flex min-h-screen items-center justify-center bg-background px-6 py-10 text-foreground">
			<section className="flex w-full max-w-2xl flex-col gap-6">
				<header className="flex flex-col gap-4">
					<div className="flex items-start justify-between gap-4">
						<div className="flex size-10 items-center justify-center text-destructive">
							<AlertTriangle className="h-6 w-6" />
						</div>
						<LanguageSwitcher />
					</div>
					<div className="flex flex-col gap-2">
						<h1 className="text-2xl font-medium">{t("errorBoundary.title")}</h1>
						<p className="max-w-2xl text-sm text-muted-foreground">{t("errorBoundary.body")}</p>
					</div>
				</header>
				<div className="flex flex-col gap-6">
					{errorId ? (
						<div className="text-sm">
							<span className="text-muted-foreground">{t("errorBoundary.errorId")} </span>
							<code className="font-mono text-foreground">{errorId}</code>
						</div>
					) : null}

					{isDev && error ? (
						<div className="flex flex-col gap-3">
							<p className="text-sm font-medium text-destructive">
								{t("errorBoundary.devDetails")}
							</p>
							<pre className="max-h-80 overflow-auto rounded-lg border border-border bg-card p-4 font-mono text-xs leading-6 text-muted-foreground">
								{error.message}
								{error.stack ? `\n\nStack trace:\n${error.stack}` : ""}
							</pre>
						</div>
					) : null}

					<div className="flex flex-wrap gap-3">
						<Button onClick={resetError}>
							<RotateCcw />
							{t("errorBoundary.reload")}
						</Button>
						<Button
							variant="outline"
							onClick={() => {
								window.location.hash = "#/";
								window.location.reload();
							}}
						>
							<Home />
							{t("errorBoundary.home")}
						</Button>
					</div>
				</div>
			</section>
		</div>
	);
};

export const ErrorBoundary: React.FC<ErrorBoundaryProps> = ({ children, ...props }) => {
	return <ErrorBoundaryClass {...props}>{children}</ErrorBoundaryClass>;
};
