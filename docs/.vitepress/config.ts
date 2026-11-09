import { defineConfig } from "vitepress";

const repository = "https://github.com/moriyoshi/gpui-go";

export default defineConfig({
  title: "gpui-go",
  description: "Native desktop UIs in Go, rendered by GPUI.",
  lang: "en",
  cleanUrls: true,
  head: [
    ["link", { rel: "preconnect", href: "https://fonts.googleapis.com" }],
    ["link", { rel: "preconnect", href: "https://fonts.gstatic.com", crossorigin: "" }],
    [
      "link",
      {
        rel: "stylesheet",
        href: "https://fonts.googleapis.com/css2?family=Titillium+Web:ital,wght@0,400;0,600;0,700;1,400;1,600&display=swap",
      },
    ],
  ],
  themeConfig: {
    search: { provider: "local" },
    socialLinks: [{ icon: "github", link: repository }],
    editLink: { pattern: `${repository}/edit/main/docs/:path` },
    nav: [
      { text: "Guide", link: "/guide/introduction" },
      { text: "Elements", link: "/reference/" },
    ],
    sidebar: [
      {
        text: "Start here",
        items: [
          { text: "Introduction", link: "/guide/introduction" },
          { text: "Getting started", link: "/guide/getting-started" },
          { text: "Prebuilt modules", link: "/guide/prebuilt-modules" },
        ],
      },
      {
        text: "Building interfaces",
        items: [
          { text: "Concepts", link: "/guide/concepts" },
          { text: "Reactive components", link: "/guide/reactive" },
          { text: "GoX templates", link: "/guide/gox" },
          { text: "Zed editor", link: "/guide/editor" },
        ],
      },
      {
        text: "Element reference",
        items: [
          { text: "Overview", link: "/reference/" },
          { text: "Container", link: "/reference/div" },
          { text: "Text", link: "/reference/text" },
          { text: "Text input", link: "/reference/text-input" },
          { text: "Image", link: "/reference/image" },
          { text: "Canvas", link: "/reference/canvas" },
          { text: "Uniform list", link: "/reference/uniform-list" },
          { text: "Node methods", link: "/reference/node-methods" },
          { text: "Style properties", link: "/reference/style" },
          { text: "Events", link: "/reference/events" },
          { text: "Reactive nodes", link: "/reference/reactive-nodes" },
          { text: "GoX tags", link: "/reference/gox-tags" },
          { text: "Limits", link: "/guide/limits" },
        ],
      },
      {
        text: "Testing",
        items: [
          { text: "Overview", link: "/testing/" },
          { text: "Set up your application", link: "/testing/setup" },
          { text: "Writing scenarios", link: "/testing/writing-scenarios" },
          { text: "Starlark scenarios", link: "/testing/starlark" },
          { text: "Headless testing", link: "/testing/headless" },
          { text: "API reference", link: "/testing/api" },
        ],
      },
    ],
    footer: {
      message: "Released under the Apache License 2.0. The optional Zed editor component is GPL-3.0-or-later.",
    },
  },
});
