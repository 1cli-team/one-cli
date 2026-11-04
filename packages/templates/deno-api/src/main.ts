const port = Number(Deno.env.get("PORT") ?? 3000);

Deno.serve({ port }, () => Response.json({ status: "ok" }));
