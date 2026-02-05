import React from "react";
import Layout from "@theme/Layout";
import Link from "@docusaurus/Link";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";

function HomepageHeader() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <header className="hero hero--primary">
      <div className="container">
        <h1 className="hero__title">{siteConfig.title}</h1>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
        <div className="heroButtons">
          <Link className="button button--secondary button--lg" to="/docs/intro">
            Get Started
          </Link>
          <Link
            className="button button--secondary button--outline button--lg"
            to="/docs/category/api"
          >
            API Reference
          </Link>
        </div>
      </div>
    </header>
  );
}

export default function Home(): React.JSX.Element {
  const { siteConfig } = useDocusaurusContext();
  return (
    <Layout title="Home" description={siteConfig.tagline}>
      <HomepageHeader />
      <main>
        <section style={{ padding: "2rem 0" }}>
          <div className="container">
            <div className="row">
              <div className="col col--4">
                <div style={{ textAlign: "center", padding: "1rem" }}>
                  <h3>Extensive Archive</h3>
                  <p>
                    Access over 4000 high-quality anime-style images with
                    powerful tag-based filtering.
                  </p>
                </div>
              </div>
              <div className="col col--4">
                <div style={{ textAlign: "center", padding: "1rem" }}>
                  <h3>Simple Integration</h3>
                  <p>
                    Easy-to-use REST API with comprehensive OpenAPI
                    documentation. Get started in minutes.
                  </p>
                </div>
              </div>
              <div className="col col--4">
                <div style={{ textAlign: "center", padding: "1rem" }}>
                  <h3>Advanced Filtering</h3>
                  <p>
                    Filter by tags, orientation, resolution, artists, and more.
                    Sort by date, popularity, or randomly.
                  </p>
                </div>
              </div>
            </div>
          </div>
        </section>
      </main>
    </Layout>
  );
}
