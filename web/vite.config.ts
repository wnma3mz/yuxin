import { defineConfig } from "vitest/config";

export default defineConfig({
  base: "./",
  plugins: [{
    name: "production-content-security-policy",
    transformIndexHtml: {
      order: "post",
      handler(_html, context) {
        if (context.server) return [];
        return [{
          tag: "meta",
          attrs: {
            "http-equiv": "Content-Security-Policy",
            content: "default-src 'self'; connect-src 'self' https://nubeymzysjmlwgzjpstl.supabase.co; img-src 'self' data:; script-src 'self'; style-src 'self'; base-uri 'none'; form-action 'self'; object-src 'none'; upgrade-insecure-requests",
          },
          injectTo: "head-prepend",
        }];
      },
    },
  }],
  test: {
    include: ["tests/**/*.test.ts"],
  },
});
