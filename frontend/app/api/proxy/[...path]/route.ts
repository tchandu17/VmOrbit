import { NextRequest, NextResponse } from "next/server";

const BACKEND = process.env.BACKEND_INTERNAL_URL || "http://backend:8080";

async function handler(req: NextRequest, { params }: { params: Promise<{ path: string[] }> }) {
  const { path } = await params;
  const url = `${BACKEND}/${path.join("/")}${req.nextUrl.search}`;

  const headers: Record<string, string> = {};

  const auth = req.headers.get("Authorization");
  if (auth) headers["Authorization"] = auth;

  // Only set Content-Type for requests that have a body
  const hasBody = req.method !== "GET" && req.method !== "HEAD" && req.method !== "DELETE";
  if (hasBody) headers["Content-Type"] = "application/json";

  const body = hasBody ? await req.text() : undefined;

  let res: Response;
  try {
    res = await fetch(url, {
      method: req.method,
      headers,
      body,
    });
  } catch (err) {
    return new NextResponse(
      JSON.stringify({ success: false, error: "Backend unreachable" }),
      { status: 502, headers: { "Content-Type": "application/json" } }
    );
  }

  // 204 No Content — return empty response, don't try to read body
  if (res.status === 204) {
    return new NextResponse(null, { status: 204 });
  }

  const data = await res.text();
  const contentType = res.headers.get("Content-Type") ?? "application/json";

  return new NextResponse(data, {
    status: res.status,
    headers: { "Content-Type": contentType },
  });
}

export { handler as GET, handler as POST, handler as PUT, handler as DELETE, handler as PATCH };
