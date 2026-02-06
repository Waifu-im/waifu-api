import { themes as prismThemes } from "prism-react-renderer";
import type { Config } from "@docusaurus/types";
import type * as Preset from "@docusaurus/preset-classic";
import type * as OpenApiPlugin from "docusaurus-plugin-openapi-docs";
import * as fs from "fs";
import * as path from "path";

function getApiMajorVersion(): string {
  try {
    const specPath = path.join(__dirname, "../openapi/WaifuApi.Web.json");
    const spec = JSON.parse(fs.readFileSync(specPath, "utf-8"));
    const version = spec.info?.version || "1.0.0";
    const major = version.split(".")[0];
    return `v${major}`;
  } catch {
    return "v1";
  }
}

const appName = process.env.APP_NAME!;
const appTagline = process.env.APP_TAGLINE!;
const frontendUrl = process.env.FRONTEND_URL!;
const githubUrl = process.env.GITHUB_URL!;

const config: Config = {
  title: appName,
  tagline: appTagline,
  favicon: "img/favicon.png",

  url: process.env.DOCS_URL!,
  baseUrl: process.env.DOCS_BASE_URL!,

  organizationName: process.env.DOCS_ORG_NAME!,
  projectName: process.env.DOCS_PROJECT_NAME!,

  customFields: {
    apiVersion: getApiMajorVersion(),
    apiBaseUrl: process.env.API_BASE_URL!,
    appName,
    frontendUrl,
    githubUrl,
  },

  onBrokenLinks: "warn",

  markdown: {
    hooks: {
      onBrokenMarkdownLinks: "warn",
    },
  },

  i18n: {
    defaultLocale: "en",
    locales: ["en"],
  },

  presets: [
    [
      "classic",
      {
        docs: {
          sidebarPath: "./sidebars.ts",
          docItemComponent: "@theme/ApiItem",
        },
        blog: false,
        theme: {
          customCss: "./src/css/custom.css",
        },
      } satisfies Preset.Options,
    ],
  ],

  plugins: [
    [
      "docusaurus-plugin-openapi-docs",
      {
        id: "api",
        docsPluginId: "classic",
        config: {
          waifuApi: {
            // Generated at build time by Microsoft.Extensions.ApiDescription.Server
            // The spec is output to ../openapi/{ProjectName}.json by `dotnet build`
            specPath: "../openapi/WaifuApi.Web.json",
            outputDir: "docs/api",
            sidebarOptions: {
              groupPathsBy: "tag",
            },
          },
        },
      },
    ],
  ],

  themes: ["docusaurus-theme-openapi-docs"],

  headTags: [
    {
      tagName: "meta",
      attributes: {
        property: "og:image",
        content: "img/favicon.png",
      },
    },
    {
      tagName: "meta",
      attributes: {
        name: "twitter:card",
        content: "summary",
      },
    },
    {
      tagName: "meta",
      attributes: {
        name: "twitter:image",
        content: "img/favicon.png",
      },
    },
  ],

  themeConfig: {
    metadata: [
      {
        name: "description",
        content: appTagline,
      },
      {
        property: "og:description",
        content: appTagline,
      },
    ],
    colorMode: {
      defaultMode: "dark",
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: appName,
      logo: {
        alt: `${appName} Logo`,
        src: "img/favicon.png",
      },
      items: [
        {
          type: "docSidebar",
          sidebarId: "docs",
          position: "left",
          label: "Documentation",
        },
        {
          type: "docSidebar",
          sidebarId: "api",
          position: "left",
          label: "API Reference",
        },
        {
          href: frontendUrl,
          label: "Website",
          position: "right",
        },
        {
          href: githubUrl,
          position: "right",
          className: "header-github-link",
          "aria-label": "GitHub repository",
        },
      ],
    },
    footer: {
      style: "dark",
      links: [
        {
          title: "Documentation",
          items: [
            { label: "Introduction", to: "/docs/intro" },
            { label: "Getting Started", to: "/docs/getting-started" },
            { label: "Tags", to: "/docs/tags" },
            { label: "API Reference", to: "/docs/category/api" },
          ],
        },
        {
          title: "Links",
          items: [
            { label: "Website", href: frontendUrl },
            {
              label: "GitHub",
              href: githubUrl,
            },
            { label: "Support", href: `${frontendUrl}/contact/` },
          ],
        },
      ],
      copyright: `Copyright \u00a9 ${new Date().getFullYear()} ${appName}`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ["bash", "json", "python", "csharp"],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
