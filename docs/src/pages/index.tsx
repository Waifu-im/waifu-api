import React from "react";
import Layout from "@theme/Layout";
import Link from "@docusaurus/Link";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";
import useBaseUrl from "@docusaurus/useBaseUrl";

const features = [
  {
    title: "Extensive Archive",
    description:
      "Over 4,000 curated anime images, continuously growing with community contributions.",
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
        <circle cx="8.5" cy="8.5" r="1.5" />
        <polyline points="21 15 16 10 5 21" />
      </svg>
    ),
  },
  {
    title: "Simple Integration",
    description:
      "Clean REST API with full OpenAPI specification. Get started with a single HTTP request.",
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <polyline points="16 18 22 12 16 6" />
        <polyline points="8 6 2 12 8 18" />
      </svg>
    ),
  },
  {
    title: "Advanced Filtering",
    description:
      "Filter by tags, artists, resolution, orientation, file size, and more.",
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3" />
      </svg>
    ),
  },
  {
    title: "Favorites & Albums",
    description:
      "Authenticate to save favorites and manage your personal collections.",
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z" />
      </svg>
    ),
  },
];

function HomepageHero() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <header className="homepage-hero">
      <div className="container">
        <div className="homepage-hero__inner">
          <img
            src={useBaseUrl("/img/favicon.png")}
            alt={siteConfig.title}
            className="homepage-hero__logo"
          />
          <h1 className="homepage-hero__title">{siteConfig.title}</h1>
          <p className="homepage-hero__tagline">{siteConfig.tagline}</p>
          <div className="homepage-hero__actions">
            <Link className="button button--primary button--lg" to="/docs/getting-started">
              Get Started
            </Link>
            <Link className="button button--outline button--secondary button--lg" to="/docs/category/api">
              API Reference
            </Link>
          </div>
        </div>
      </div>
    </header>
  );
}

function Features() {
  return (
    <section className="homepage-features">
      <div className="container">
        <div className="homepage-features__grid">
          {features.map((feature, idx) => (
            <div key={idx} className="homepage-features__card">
              <div className="homepage-features__icon">{feature.icon}</div>
              <div>
                <h3 className="homepage-features__title">{feature.title}</h3>
                <p className="homepage-features__desc">{feature.description}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function BottomCTA() {
  return (
    <section className="homepage-cta">
      <div className="container">
        <div className="homepage-cta__inner">
          <h2 className="homepage-cta__title">Ready to get started?</h2>
          <p className="homepage-cta__desc">
            Read the documentation or browse the full API reference.
          </p>
          <div className="homepage-cta__actions">
            <Link className="button button--primary button--lg" to="/docs/getting-started">
              Read the Docs
            </Link>
            <Link className="button button--outline button--secondary button--lg" to="/docs/category/api">
              API Reference
            </Link>
          </div>
        </div>
      </div>
    </section>
  );
}

export default function Home(): React.JSX.Element {
  const { siteConfig } = useDocusaurusContext();
  return (
    <Layout title="Home" description={siteConfig.tagline}>
      <HomepageHero />
      <main>
        <Features />
        <BottomCTA />
      </main>
    </Layout>
  );
}
