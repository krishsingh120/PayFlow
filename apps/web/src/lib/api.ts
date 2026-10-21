// API Client for PayFlow Frontend
// Calls pass through the Node.js API Gateway (/api/*) which handles rate-limiting and CORS.

export interface Payment {
  id: string;
  amount: number; // in paise
  currency: string;
  status: 'CREATED' | 'PENDING' | 'SUCCEEDED' | 'FAILED' | 'UNKNOWN';
  description?: string;
  idempotency_key: string;
  provider_ref?: string;
  failure_reason?: string;
  metadata?: Record<string, any>;
  created_at: string;
  updated_at: string;
}

export interface CreatePaymentPayload {
  amount: number; // paise
  currency: string;
  description?: string;
  metadata?: Record<string, any>;
}

export interface ListPaymentsResponse {
  data: Payment[];
  next_cursor?: string;
  total: number;
}

export interface ApiResponse<T> {
  data?: T;
  status: number;
  isReplay?: boolean;
  error?: string;
  message?: string;
  latencyMs: number;
}

const API_BASE = '/api';

export async function createPayment(
  idempotencyKey: string,
  payload: CreatePaymentPayload
): Promise<ApiResponse<Payment>> {
  const start = performance.now();
  try {
    const res = await fetch(`${API_BASE}/v1/payments`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Idempotency-Key': idempotencyKey,
      },
      body: JSON.stringify(payload),
    });

    const latencyMs = Math.round(performance.now() - start);
    const json = await res.json();

    if (!res.ok) {
      return {
        status: res.status,
        error: json.error || 'Error',
        message: json.message || 'Payment request failed',
        latencyMs,
      };
    }

    return {
      status: res.status,
      isReplay: res.status === 200,
      data: json as Payment,
      latencyMs,
    };
  } catch (err: any) {
    const latencyMs = Math.round(performance.now() - start);
    return {
      status: 0,
      error: 'NetworkError',
      message: err.message || 'Failed to connect to gateway',
      latencyMs,
    };
  }
}

export async function getPayment(id: string): Promise<ApiResponse<Payment>> {
  const start = performance.now();
  try {
    const res = await fetch(`${API_BASE}/v1/payments/${id}`);
    const latencyMs = Math.round(performance.now() - start);
    const json = await res.json();

    if (!res.ok) {
      return {
        status: res.status,
        error: json.error || 'Error',
        message: json.message || 'Payment not found',
        latencyMs,
      };
    }

    return {
      status: res.status,
      data: json as Payment,
      latencyMs,
    };
  } catch (err: any) {
    return {
      status: 0,
      error: 'NetworkError',
      message: err.message,
      latencyMs: Math.round(performance.now() - start),
    };
  }
}

export async function listPayments(params?: {
  limit?: number;
  cursor?: string;
  status?: string;
}): Promise<ApiResponse<ListPaymentsResponse>> {
  const start = performance.now();
  try {
    const query = new URLSearchParams();
    if (params?.limit) query.set('limit', params.limit.toString());
    if (params?.cursor) query.set('cursor', params.cursor);
    if (params?.status) query.set('status', params.status);

    const res = await fetch(`${API_BASE}/v1/payments?${query.toString()}`);
    const latencyMs = Math.round(performance.now() - start);
    const json = await res.json();

    if (!res.ok) {
      return {
        status: res.status,
        error: json.error,
        message: json.message,
        latencyMs,
      };
    }

    return {
      status: res.status,
      data: json as ListPaymentsResponse,
      latencyMs,
    };
  } catch (err: any) {
    return {
      status: 0,
      error: 'NetworkError',
      message: err.message,
      latencyMs: Math.round(performance.now() - start),
    };
  }
}

export async function checkGatewayHealth(): Promise<{ ok: boolean; upstream?: string; latencyMs: number }> {
  const start = performance.now();
  try {
    // Gateway health is served at /health
    const res = await fetch('/health');
    const latencyMs = Math.round(performance.now() - start);
    if (res.ok) {
      const json = await res.json();
      return { ok: true, upstream: json.upstream, latencyMs };
    }
    return { ok: false, latencyMs };
  } catch {
    return { ok: false, latencyMs: Math.round(performance.now() - start) };
  }
}

export function formatINR(paise: number): string {
  const rupees = paise / 100;
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    maximumFractionDigits: 2,
  }).format(rupees);
}

export interface Refund {
  id: string;
  payment_id: string;
  amount: number;
  status: 'PENDING' | 'SUCCEEDED' | 'FAILED';
  reason?: string;
  idempotency_key?: string;
  created_at: string;
  updated_at: string;
}

export async function createRefund(
  paymentId: string,
  payload: { amount: number; reason?: string },
  idempotencyKey?: string
): Promise<ApiResponse<Refund>> {
  const start = performance.now();
  try {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (idempotencyKey) {
      headers['Idempotency-Key'] = idempotencyKey;
    }
    const res = await fetch(`${API_BASE}/v1/payments/${paymentId}/refunds`, {
      method: 'POST',
      headers,
      body: JSON.stringify(payload),
    });
    const latencyMs = Math.round(performance.now() - start);
    const json = await res.json();
    if (!res.ok) {
      return {
        status: res.status,
        error: json.error || 'Error',
        message: json.message || 'Refund failed',
        latencyMs,
      };
    }
    return {
      status: res.status,
      isReplay: res.headers.get('Idempotent-Replay') === 'true' || res.status === 200,
      data: json as Refund,
      latencyMs,
    };
  } catch (err: any) {
    return {
      status: 0,
      error: 'NetworkError',
      message: err.message,
      latencyMs: Math.round(performance.now() - start),
    };
  }
}

export async function listRefunds(paymentId: string): Promise<ApiResponse<Refund[]>> {
  const start = performance.now();
  try {
    const res = await fetch(`${API_BASE}/v1/payments/${paymentId}/refunds`);
    const latencyMs = Math.round(performance.now() - start);
    const json = await res.json();
    if (!res.ok) {
      return {
        status: res.status,
        error: json.error,
        message: json.message,
        latencyMs,
      };
    }
    return {
      status: res.status,
      data: json as Refund[],
      latencyMs,
    };
  } catch (err: any) {
    return {
      status: 0,
      error: 'NetworkError',
      message: err.message,
      latencyMs: Math.round(performance.now() - start),
    };
  }
}

