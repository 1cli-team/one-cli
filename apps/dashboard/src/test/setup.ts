import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

afterEach(() => cleanup());

class ResizeObserverStub {
	disconnect() {}
	observe() {}
	unobserve() {}
}

Object.defineProperty(globalThis, "ResizeObserver", {
	configurable: true,
	writable: true,
	value: ResizeObserverStub,
});

Object.defineProperty(window, "matchMedia", {
	configurable: true,
	writable: true,
	value: (query: string) => ({
		matches: false,
		media: query,
		onchange: null,
		addEventListener() {},
		removeEventListener() {},
		addListener() {},
		removeListener() {},
		dispatchEvent: () => false,
	}),
});

HTMLElement.prototype.scrollIntoView = () => {};
HTMLElement.prototype.hasPointerCapture = () => false;
HTMLElement.prototype.releasePointerCapture = () => {};
