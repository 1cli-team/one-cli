import { Component, type PropsWithChildren } from "react";

export class ErrorBoundary extends Component<PropsWithChildren, { failed: boolean }> {
	state = { failed: false };
	static getDerivedStateFromError() {
		return { failed: true };
	}
	componentDidCatch(error: Error) {
		console.error(error);
	}
	render() {
		return this.state.failed ? (
			<div role="alert">
				<p>页面出现错误 / Something went wrong</p>
				<button onClick={() => window.location.reload()}>重新加载 / Reload</button>
			</div>
		) : (
			this.props.children
		);
	}
}
