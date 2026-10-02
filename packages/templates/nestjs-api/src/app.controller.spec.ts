import { AppController } from "./app.controller";

describe("AppController", () => {
  it("reports a live process", () => {
    const result = new AppController().health();
    expect(result.status).toBe("ok");
    expect(result.uptime).toBeGreaterThanOrEqual(0);
  });
});
