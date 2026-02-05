import { themes as prismThemes } from "prism-react-renderer";
import type { Config } from "@docusaurus/types";
import type * as Preset from "@docusaurus/preset-classic";
import type * as OpenApiPlugin from "docusaurus-plugin-openapi-docs";

const config: Config = {
  title: "Waifu.im",
  tagline: "The versatile waifu image provider",
  favicon: "img/favicon.ico",

  // GitHub Pages deployment config
  // If using a custom domain (e.g. docs.waifu.im), set url to "https://docs.waifu.im" and baseUrl to "/"
  url: "https://waifu-im.github.io",
  baseUrl: "/waifu-api/",

  organizationName: "Waifu-im",
  projectName: "waifu-api",

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
          } satisfies OpenApiPlugin.OpenApiDocConfig,
        },
      },
    ],
  ],

  themes: ["docusaurus-theme-openapi-docs"],

  themeConfig: {
    colorMode: {
      defaultMode: "dark",
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: "Waifu.im",
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
          href: "https://waifu.im",
          label: "Website",
          position: "right",
        },
        {
          href: "https://github.com/Waifu-im/waifu-api",
          label: "GitHub",
          position: "right",
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
            { label: "API Reference", to: "/docs/category/api" },
          ],
        },
        {
          title: "Links",
          items: [
            { label: "Website", href: "https://waifu.im" },
            {
              label: "GitHub",
              href: "https://github.com/Waifu-im/waifu-api",
            },
            { label: "Support", href: "https://waifu.im/contact/" },
          ],
        },
      ],
      copyright: `Copyright \u00a9 ${new Date().getFullYear()} Waifu.im`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ["bash", "json", "csharp"],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
