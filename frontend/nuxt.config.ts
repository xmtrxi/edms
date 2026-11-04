import tailwindcss from "@tailwindcss/vite";

export default defineNuxtConfig({
  compatibilityDate: "2025-07-15",
  ssr: false,
  devtools: { enabled: true },

  // Nuxt 4.6.4 resolves an unset devServer.host to {} (untyped 2.0.0), which
  // crashes `nuxt dev` with "options.hostname.toLowerCase is not a function".
  devServer: {
    host: "localhost",
  },

  modules: [
    "@nuxt/fonts",
    "@nuxt/scripts",
    "@nuxt/icon",
    "shadcn-nuxt",
    "@pinia/nuxt",
    "@comark/nuxt",
  ],

  imports: {
    dirs: ["./services"],
  },

  css: ["~/assets/css/tailwind.css"],

  vite: {
    plugins: [tailwindcss()],
  },

  shadcn: {
    prefix: "",
    componentDir: "./app/components/ui",
  },

  experimental: {
    typedPages: true,
  },

  runtimeConfig: {
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE ?? "http://localhost:8080/api",
      // Shown until the API reports its APP_NAME.
      appName: process.env.NUXT_PUBLIC_APP_NAME ?? "Lumen DMS",
    },
  },

  fonts: {
    families: [
      {
        name: "IBM Plex Sans",
        provider: "google",
        weights: [400, 500, 600],
      },
      {
        name: "IBM Plex Mono",
        provider: "google",
        weights: [400, 500],
      },
    ],
  },
});
