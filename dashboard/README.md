# HiveMind dashboard

The control-plane UI — Next.js 16, App Router, static export (`output: "export"`, no Node server at runtime). Deployed to S3 + CloudFront, not Vercel: the API this talks to (`internal/dashboardapi`) is a Lambda behind a Function URL, and static hosting is the whole point (see [`docs/ARCHITECTURE.md`](../docs/ARCHITECTURE.md) § "Why the boundaries are where they are").

For everything else — what the ten pages do, how deploy actually works, the i18n/theming conventions — see the root [`README.md`](../README.md), [`docs/CONTROL_PLANE.md`](../docs/CONTROL_PLANE.md), and [`docs/DEPLOYMENT.md`](../docs/DEPLOYMENT.md#dashboard-static-site). [`AGENTS.md`](AGENTS.md) has the one thing specific to writing code in this folder: Next.js 16 has breaking changes since most training data, so check `node_modules/next/dist/docs/` before assuming an API.

## Local dev

```bash
npm ci
echo 'NEXT_PUBLIC_DASHBOARD_API_URL=http://localhost:8090' > .env.local   # or a live dashboard-api URL
npm run dev                         # http://localhost:3000
```

```bash
npm run build     # static export to dashboard/out/ — what actually ships
npx tsc --noEmit  # type-check
npx eslint app components lib
```
