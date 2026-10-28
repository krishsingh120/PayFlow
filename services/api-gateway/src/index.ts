import express, { Request, Response, NextFunction } from 'express';
import cors from 'cors';
import morgan from 'morgan';
import rateLimit from 'express-rate-limit';
import { createProxyMiddleware } from 'http-proxy-middleware';
import type { Socket } from 'net';
import { ServerResponse } from 'http';

// ── Configuration ─────────────────────────────────────────────────────────────
const PORT = parseInt(process.env['GATEWAY_PORT'] ?? '3000', 10);
const PAYMENT_SERVICE_URL = process.env['PAYMENT_SERVICE_URL'] ?? 'http://localhost:8080';

const app = express();

// ── Global Middleware ─────────────────────────────────────────────────────────

// CORS — allow the React dev server and any configured origin.
app.use(cors({
  origin: process.env['ALLOWED_ORIGINS']?.split(',') ?? '*',
  methods: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS'],
  allowedHeaders: ['Content-Type', 'Authorization', 'Idempotency-Key', 'X-Request-ID'],
}));

// Structured HTTP request logging.
app.use(morgan('combined'));

// Rate limiting — 200 req/min per IP. Configurable without code changes.
const limiter = rateLimit({
  windowMs: 60 * 1000,
  max: 200,
  standardHeaders: true,
  legacyHeaders: false,
  message: { error: 'Too many requests, please try again later.' },
});
app.use(limiter);

// ── Gateway Health ────────────────────────────────────────────────────────────
app.get('/health', (_req: Request, res: Response) => {
  res.json({
    status: 'ok',
    service: 'api-gateway',
    timestamp: new Date().toISOString(),
    upstream: PAYMENT_SERVICE_URL,
  });
});

// ── Proxy: /api/* → Payment Service ──────────────────────────────────────────
// The gateway is intentionally thin. All payment business logic lives in Go.
// We strip the /api prefix so the Go service sees clean /v1/... paths.
app.use(
  '/api',
  createProxyMiddleware({
    target: PAYMENT_SERVICE_URL,
    changeOrigin: true,
    pathRewrite: { '^/api': '' },
    on: {
      error: (err: Error, _req: Request, res: Response | Socket | ServerResponse) => {
        console.error('[gateway] proxy error:', err.message);
        if (res instanceof ServerResponse) {
          res.writeHead(502, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({
            error: 'Bad Gateway',
            message: 'Payment service is temporarily unavailable.',
          }));
        }
      },
    },
  })
);

// ── 404 Handler ───────────────────────────────────────────────────────────────
app.use((_req: Request, res: Response) => {
  res.status(404).json({ error: 'Not Found' });
});

// ── Global Error Handler ──────────────────────────────────────────────────────
app.use((err: Error, _req: Request, res: Response, _next: NextFunction) => {
  console.error('[gateway] unhandled error:', err);
  res.status(500).json({ error: 'Internal Server Error' });
});

// ── Start ─────────────────────────────────────────────────────────────────────
if (process.env['NODE_ENV'] !== 'test') {
  app.listen(PORT, () => {
    console.log(`[gateway] listening on :${PORT} → proxying to ${PAYMENT_SERVICE_URL}`);
  });
}

export default app;
