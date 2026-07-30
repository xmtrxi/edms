import tailwindcss from "@tailwindcss/vite";

export default defineNuxtConfig({
  compatibilityDate: "2025-07-15",
  devtools: { enabled: true },

  modules: ["@nuxt/fonts", "@nuxt/scripts", "@nuxt/icon", "shadcn-nuxt"],

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
