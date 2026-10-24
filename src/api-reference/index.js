// @ts-check
import { createApiReference } from "https://cdn.jsdelivr.net/npm/@scalar/api-reference/esm.js";

const title = "SkyCrypt API Reference";
const description = "The SkyCrypt API provides access to player profiles and other data. It is intended for use by developers who want to integrate SkyCrypt data into their own applications or services.";

createApiReference("#app", {
  url: "/api/openapi/doc.json",
  defaultHttpClient: { targetKey: "node", clientKey: 'fetch', },
  featuredClients: ['node/fetch', 'java/nethttp'],
  favicon: "/assets/static/favicon.ico",
  hideClientButton: true,
  metaData: {
    title: title,
    description: description,
    ogDescription: description,
    ogTitle: title,
    ogImage: "/assets/static/skycrypt-api.avif",
    twitterCard: "summary_large_image",
  },
  servers: [
    { url: "https://api.shiiyu.moe", description: "Production Server" },
    { url: "https://beta-api.shiiyu.moe", description: "Beta Server" },
  ],
});
