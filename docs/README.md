# Espresso Documentation

This directory contains the VitePress documentation site for Espresso.

## Prerequisites

- Node.js 20+
- npm or yarn

## Setup

```bash
# Install dependencies
npm install

# Run development server
npm run docs:dev

# Build for production
npm run docs:build

# Preview production build
npm run docs:preview
```

## Structure

```
docs/
├── .vitepress/
│   ├── config.ts          # VitePress configuration
│   ├── theme/
│   │   ├── custom.css      # Custom styles
│   │   └── index.ts        # Theme customization
│   └── components/
│       └── Mermaid.vue     # Mermaid diagram component
├── guide/
│   ├── index.md            # Getting Started
│   ├── installation.md
│   ├── quick-start.md
│   ├── core-concepts.md
│   ├── handlers.md
│   ├── extractors.md
│   └── state.md
├── examples/
│   └── ...
├── api/
│   └── ...                 # Auto-generated API docs
├── public/
│   └── logo.png
└── index.md                # Landing page
```

## Writing Docs

### Code Examples

Use standard Markdown code blocks:

````markdown
```go
func handler(ctx context.Context, req *espresso.JSON[Req]) (Res, error) {
    return Res{Data: "example"}, nil
}
```
````

### Mermaid Diagrams

Use the Mermaid component for architecture diagrams:

```markdown
<Mermaid source="graph TB
    Request --> Middleware --> Handler --> Response" />
```

### API Generation

Run the API documentation generator:

```bash
npm run docs:gen-api
```

## Deployment

Documentation is automatically deployed to GitHub Pages on push to `main` branch.

Workflow: `.github/workflows/docs.yml`

## Local Development

1. Install dependencies: `npm install`
2. Start dev server: `npm run docs:dev`
3. Open http://localhost:5173/espresso/

## Customization

### Theme Colors

Edit `.vitepress/theme/custom.css` to change the coffee-themed colors:

```css
:root {
  --vp-c-brand-1: #8B4513;  /* Coffee Brown */
  --vp-c-brand-2: #A0522D;  /* Sienna */
}
```

### Navigation

Edit `.vitepress/config.ts` to update navigation and sidebar.

## Contributing

1. Edit `.md` files in `docs/` directory
2. Run `npm run docs:build` to verify build
3. Submit PR with changes