from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import os


class Handler(BaseHTTPRequestHandler):
    def do_GET(self) -> None:
        payload = b'{"status":"ok"}'
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)


def main() -> None:
    port = int(os.environ.get("PORT", "3000"))
    with ThreadingHTTPServer(("0.0.0.0", port), Handler) as server:
        server.serve_forever()


if __name__ == "__main__":
    main()
