import React, { useState } from "react";
import Layout from "@theme/Layout";
import Link from "@docusaurus/Link";

const CDN = "https://waifu-cdn.waifu-api.workers.dev";
const SITE = "https://waifu-api-eosin.vercel.app";

type Loaded = {
  url: string;
  caption: string;
  page: string;
};

export default function Playground(): JSX.Element {
  const [target, setTarget] = useState(`${CDN}/images?IsNsfw=False&PageSize=1`);
  const [image, setImage] = useState<Loaded | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function load() {
    setBusy(true);
    setError(null);
    setImage(null);

    // Anonymous GETs are cached for 60s at the edge, so re-running the same
    // URL returns the byte-identical response -- and with OrderBy=Random that
    // means the same image every time, which defeats the point of a playground.
    // A throwaway param gives each click its own cache key. The field still
    // shows what you typed; only the request is busted.
    let requestUrl: string;
    try {
      const u = new URL(target);
      u.searchParams.set("_", String(Date.now()));
      requestUrl = u.toString();
    } catch {
      setError("That doesn't look like a valid absolute URL.");
      setBusy(false);
      return;
    }

    try {
      const res = await fetch(requestUrl);
      if (!res.ok) {
        throw new Error(`HTTP ${res.status} — ${res.status === 400 ? "check the query parameter names" : "request failed"}`);
      }
      const body = await res.json();
      const items: any[] = body.items ?? [];
      if (items.length === 0) {
        setError("No images matched that query.");
        return;
      }
      const first = items[0];
      const tags: string[] = (first.tags ?? []).map((t: any) => t.name);
      setImage({
        url: first.url,
        page: `${SITE}/images/${first.id}`,
        caption: `#${first.id} · ${first.width}×${first.height}${
          tags.length ? ` · ${tags.join(", ")}` : ""
        }`,
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Layout title="Playground" description="Try the image endpoint and see the result.">
      <main className="container margin-vert--lg">
        <h1>Playground</h1>
        <p>
          Edit the URL and load it. The base is the CDN, so responses come from
          the edge cache and image bytes never touch the API host. Append the
          same query parameters you'd use in a real request —{" "}
          <code>IsNsfw</code>, <code>IncludedTags</code>, <code>ExcludedTags</code>,{" "}
          <code>Orientation</code>, <code>IsAnimated</code>, <code>PageSize</code>,{" "}
          <code>Page</code>.
        </p>

        <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
          <input
            value={target}
            onChange={(e) => setTarget(e.target.value)}
            spellCheck={false}
            style={{
              flex: "1 1 28rem",
              padding: "0.6rem 0.75rem",
              fontFamily: "monospace",
              fontSize: "0.85rem",
              border: "1px solid var(--ifm-color-emphasis-300)",
              borderRadius: "6px",
              background: "var(--ifm-background-surface-color)",
              color: "var(--ifm-font-color-base)",
            }}
          />
          <button
            onClick={load}
            disabled={busy}
            className="button button--primary"
            style={{ padding: "0.6rem 1.4rem" }}
          >
            {busy ? "Loading…" : "Load"}
          </button>
        </div>
        <p style={{ fontSize: "0.85rem", opacity: 0.7, marginTop: "0.6rem" }}>
          Each click requests a fresh result — the CDN's 60-second cache is
          bypassed so you get a new image, not the one you just saw. Image
          bytes are still served from the edge.
        </p>

        {error && (
          <p
            style={{
              marginTop: "1rem",
              padding: "0.75rem 1rem",
              borderRadius: "6px",
              background: "var(--ifm-color-danger-contrast-background)",
              color: "var(--ifm-color-danger-contrast-foreground)",
            }}
          >
            {error}
          </p>
        )}

        {image && (
          <figure style={{ marginTop: "1.5rem" }}>
            <a href={image.page} target="_blank" rel="noreferrer noopener">
              <img
                src={image.url}
                alt={image.caption}
                style={{
                  maxWidth: "100%",
                  maxHeight: "70vh",
                  borderRadius: "10px",
                  boxShadow: "0 8px 30px rgba(0,0,0,0.12)",
                  cursor: "pointer",
                }}
              />
            </a>
            <figcaption style={{ marginTop: "0.6rem", opacity: 0.75, fontSize: "0.9rem" }}>
              {image.caption} ·{" "}
              <a href={image.page} target="_blank" rel="noreferrer noopener">
                view on the site
              </a>
            </figcaption>
          </figure>
        )}

        <hr style={{ margin: "2.5rem 0" }} />

        <h2>Notes</h2>
        <ul>
          <li>
            Image URLs in the response already point at the CDN, so the browser
            fetches them directly — they never pass through this page.
          </li>
          <li>
            Anonymous <code>GET</code>s are cached for 60 seconds at the edge.
            Repeat a query inside that window and you'll get the cached copy
            rather than a fresh database read.
          </li>
          <li>
            Credentials belong on the API host, not the CDN — the CDN drops{" "}
            <code>X-Api-Key</code>. See{" "}
            <Link to="/docs/authentication">Authentication</Link>.
          </li>
          <li>
            Full parameter list is in{" "}
            <Link to="/docs/getting-started">Getting Started</Link>.
          </li>
        </ul>
      </main>
    </Layout>
  );
}