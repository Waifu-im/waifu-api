import React from "react";
import Layout from "@theme/Layout";
import Link from "@docusaurus/Link";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";
import CodeBlock from "@theme/CodeBlock";

const features = [
  {
    title: "Extensive Archive",
    description:
      "Over 4000 high-quality anime-style images, continuously growing with community contributions.",
    icon: (
      <svg
        width="24"
        height="24"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
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
      <svg
        width="24"
        height="24"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <polyline points="16 18 22 12 16 6" />
        <polyline points="8 6 2 12 8 18" />
      </svg>
    ),
  },
  {
    title: "Advanced Filtering",
    description:
      "Filter by tags, artists, resolution, orientation, file size, and more. Sort by date, popularity, or randomly.",
    icon: (
      <svg
        width="24"
        height="24"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3" />
      </svg>
    ),
  },
  {
    title: "Favorites & Albums",
    description:
      "Authenticate to save favorites, create custom albums, and manage your personal collections.",
    icon: (
      <svg
        width="24"
        height="24"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z" />
      </svg>
    ),
  },
];

const stats = [
  { value: "4000+", label: "Images" },
  { value: "REST", label: "API" },
  { value: "Free", label: "To Use" },
  { value: "Open", label: "Source" },
];

const exampleResponse = `{
  "items": [
    {
      "id": 8008,
      "url": "https://cdn.waifu.im/example.jpg",
      "width": 1920,
      "height": 1080,
      "tags": [{ "name": "waifu" }],
      "artists": [{ "name": "artist_name" }],
      "favorites": 42
    }
  ]
}`;

function HomepageHero() {
  const { siteConfig } = useDocusaurusContext();
  return (
    <header className="hero-section">
      <div className="hero-bg">
        <div className="hero-orb hero-orb--1" />
        <div className="hero-orb hero-orb--2" />
        <div className="hero-orb hero-orb--3" />
      </div>
      <div className="container hero-content">
        <div className="hero-badge">Anime Image API</div>
        <h1 className="hero-title">{siteConfig.title}</h1>
        <p className="hero-tagline">{siteConfig.tagline}</p>
        <div className="hero-buttons">
          <Link className="hero-btn hero-btn--primary" to="/docs/getting-started">
            Get Started
          </Link>
          <Link className="hero-btn hero-btn--secondary" to="/docs/category/api">
            API Reference
          </Link>
        </div>
      </div>
    </header>
  );
}

function Stats() {
  return (
    <section className="stats-bar">
      <div className="container">
        <div className="stats-grid">
          {stats.map((stat, idx) => (
            <div key={idx} className="stat-item">
              <span className="stat-value">{stat.value}</span>
              <span className="stat-label">{stat.label}</span>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function Features() {
  return (
    <section className="features-section">
      <div className="container">
        <div className="section-header">
          <h2 className="section-title">Built for developers</h2>
          <p className="section-subtitle">
            Everything you need to integrate anime images into your project.
          </p>
        </div>
        <div className="features-grid">
          {features.map((feature, idx) => (
            <div key={idx} className="feature-card">
              <div className="feature-icon">{feature.icon}</div>
              <h3 className="feature-title">{feature.title}</h3>
              <p className="feature-desc">{feature.description}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function CodePreview() {
  return (
    <section className="code-section">
      <div className="container">
        <div className="code-layout">
          <div className="code-text">
            <h2 className="section-title">Try it now</h2>
            <p className="section-subtitle" style={{ textAlign: "left" }}>
              Fetch a random anime image with a single request. No API key
              required for public endpoints.
            </p>
            <div className="code-request">
              <CodeBlock language="bash">
                {"curl https://api.waifu.im/images"}
              </CodeBlock>
            </div>
          </div>
          <div className="code-response">
            <div className="code-response-label">Response</div>
            <CodeBlock language="json">{exampleResponse}</CodeBlock>
          </div>
        </div>
      </div>
    </section>
  );
}

function CallToAction() {
  return (
    <section className="cta-section">
      <div className="container">
        <div className="cta-inner">
          <h2 className="cta-title">Ready to get started?</h2>
          <p className="cta-desc">
            Start building with the Waifu.im API in minutes.
          </p>
          <div className="hero-buttons">
            <Link
              className="hero-btn hero-btn--primary"
              to="/docs/getting-started"
            >
              Read the Docs
            </Link>
            <Link
              className="hero-btn hero-btn--secondary"
              href="https://github.com/Waifu-im/waifu-api"
            >
              View on GitHub
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
        <Stats />
        <Features />
        <CodePreview />
        <CallToAction />
      </main>
    </Layout>
  );
}
