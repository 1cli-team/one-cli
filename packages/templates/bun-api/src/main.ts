const port = Number(process.env.PORT ?? 3000);

Bun.serve({
  port,
  fetch() {
    return Response.json({ status: "ok" });
  },
});
