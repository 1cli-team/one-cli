import type { INestApplication } from "@nestjs/common";
import { Test } from "@nestjs/testing";
import request from "supertest";
import { AppModule } from "../src/app.module";

describe("Starter API", () => {
  let app: INestApplication;
  beforeAll(async () => {
    const module = await Test.createTestingModule({
      imports: [AppModule],
    }).compile();
    app = module.createNestApplication();
    await app.init();
  });
  afterAll(async () => {
    await app.close();
  });
  it("starts without database or authentication configuration", async () => {
    await request(app.getHttpServer())
      .get("/")
      .expect(200)
      .expect({ name: "nestjs-api", version: "0.1.0" });
    const response = await request(app.getHttpServer())
      .get("/health")
      .expect(200);
    expect(response.body.status).toBe("ok");
    expect(response.body).not.toHaveProperty("database");
  });
  it("does not include login", async () => {
    await request(app.getHttpServer())
      .post("/auth/login")
      .send({ code: "demo" })
      .expect(404);
  });
  it.each(["/users", "/api/v1/users"])(
    "does not include business routes: %s",
    async (path) => {
      await request(app.getHttpServer()).get(path).expect(404);
    },
  );
});
