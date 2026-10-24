// Types for the CDN import in index.js, which TypeScript can't resolve from a URL.
declare module "https://cdn.jsdelivr.net/npm/@scalar/api-reference/esm.js" {
  import type { AnyApiReferenceConfiguration } from "@scalar/types/api-reference";

  export function createApiReference(element: string | HTMLElement, configuration: AnyApiReferenceConfiguration): unknown;
}
