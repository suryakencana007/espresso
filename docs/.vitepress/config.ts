import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Espresso',
  description: 'Production-grade HTTP routing framework for Go',
  base: '/espresso/',
  cleanUrls: true,
  lastUpdated: true,

  head: [
    ['link', { rel: 'icon', href: '/logo.png', type: 'image/png' }],
    ['meta', { name: 'theme-color', content: '#8B4513' }],
    ['meta', { property: 'og:title', content: 'Espresso | HTTP Routing Framework for Go' }],
    ['meta', { property: 'og:description', content: 'Production-grade HTTP routing framework for Go, inspired by Axum and Tower' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:url', content: 'https://suryakencana007.github.io/espresso/' }],
    ['meta', { property: 'og:image', content: 'https://suryakencana007.github.io/espresso/logo.png' }],
  ],

markdown: {
    lineNumbers: true
  },

  ignoreDeadLinks: true,

  themeConfig: {
    logo: '/logo.png',
    siteTitle: 'Espresso',

    nav: [
      { text: 'Guide', link: '/guide/', activeMatch: '/guide/' },
      { text: 'Examples', link: '/examples/', activeMatch: '/examples/' },
      { text: 'API', link: '/api/', activeMatch: '/api/' },
      {
        text: 'v1.1.0',
        items: [
          { text: 'Changelog', link: 'https://github.com/suryakencana007/espresso/releases' },
          { text: 'Contributing', link: 'https://github.com/suryakencana007/espresso/blob/main/CONTRIBUTING.md' }
        ]
      },
      {
        text: 'GitHub',
        link: 'https://github.com/suryakencana007/espresso'
      }
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Getting Started',
          collapsed: false,
          items: [
            { text: 'Introduction', link: '/guide/' },
            { text: 'Installation', link: '/guide/installation' },
            { text: 'Quick Start', link: '/guide/quick-start' }
          ]
        },
        {
          text: 'Core Concepts',
          collapsed: false,
          items: [
            { text: 'Architecture', link: '/guide/core-concepts' },
            { text: 'Routing', link: '/guide/routing' },
            { text: 'Handlers', link: '/guide/handlers' },
            { text: 'Extractors', link: '/guide/extractors' },
            { text: 'State & DI', link: '/guide/state' }
          ]
        },
        {
          text: 'Middleware',
          collapsed: false,
          items: [
            { text: 'Overview', link: '/guide/middleware/' },
            { text: 'HTTP Middleware', link: '/guide/middleware/http' },
            { text: 'Service Layers', link: '/guide/middleware/service' }
          ]
        },
        {
          text: 'Advanced',
          collapsed: true,
          items: [
            { text: 'Response Types', link: '/guide/response' },
            { text: 'Object Pooling', link: '/guide/pooling' }
          ]
        }
      ],
      '/examples/': [
        {
          text: 'Examples',
          items: [
            { text: 'Overview', link: '/examples/' },
            { text: 'Basic REST API', link: '/examples/basic-api' },
            { text: 'Middleware Stack', link: '/examples/middleware-stack' },
            { text: 'State Management', link: '/examples/state-management' },
            { text: 'Production Setup', link: '/examples/production' }
          ]
        }
      ],
'/api/': [
        {
          text: 'API Reference',
          items: [
            { text: 'Overview', link: '/api/' },
            { text: 'Espresso (Core)', link: '/api/espresso' },
            { text: 'Extractor', link: '/api/extractor' },
            { text: 'Middleware - HTTP', link: '/api/middleware-http' },
            { text: 'Middleware - Service', link: '/api/middleware-service' },
            { text: 'State', link: '/api/state' },
            { text: 'Pool', link: '/api/pool' }
          ]
        }
      ]
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/suryakencana007/espresso' }
    ],

    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2024-present Surya Kencana'
    },

    editLink: {
      pattern: 'https://github.com/suryakencana007/espresso/edit/main/docs/:path',
      text: 'Edit this page on GitHub'
    },

    search: {
      provider: 'local'
    },

    outline: {
      level: [2, 3],
      label: 'On this page'
    }
  },

  vite: {
    resolve: {
      alias: {
        '@': '/docs/.vitepress'
      }
    }
  }
})