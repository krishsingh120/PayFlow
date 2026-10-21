import React, { useState } from 'react';
import { X, RefreshCw, Send, CheckCircle2, AlertCircle } from 'lucide-react';
import { createPayment } from '../lib/api';
import type { Payment, ApiResponse } from '../lib/api';

interface CreatePaymentModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

export const CreatePaymentModal: React.FC<CreatePaymentModalProps> = ({ isOpen, onClose, onSuccess }) => {
  const [rupees, setRupees] = useState('500');
  const [currency, setCurrency] = useState('INR');
  const [description, setDescription] = useState('Order #ORD-7890');
  const [orderId, setOrderId] = useState('ord_7890');
  const [idempotencyKey, setIdempotencyKey] = useState<string>(() => crypto.randomUUID());

  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<ApiResponse<Payment> | null>(null);

  if (!isOpen) return null;

  const paise = Math.round(parseFloat(rupees || '0') * 100);

  const generateNewKey = () => {
    setIdempotencyKey(crypto.randomUUID());
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setResult(null);

    const res = await createPayment(idempotencyKey, {
      amount: paise,
      currency: currency.trim().toUpperCase(),
      description,
      metadata: { order_id: orderId, source: 'web_dashboard' },
    });

    setLoading(false);
    setResult(res);

    if (res.status === 201 || res.status === 200) {
      onSuccess();
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 20 }}>
          <div>
            <h3 style={{ fontSize: 18, fontWeight: 600, color: '#F8FAFC' }}>Initiate Payment</h3>
            <p style={{ fontSize: 13, color: 'var(--text-dim)' }}>
              POST /v1/payments via Node.js API Gateway
            </p>
          </div>
          <button
            onClick={onClose}
            style={{
              background: 'none',
              border: 'none',
              color: 'var(--text-dim)',
              cursor: 'pointer',
              display: 'flex',
            }}
          >
            <X size={20} />
          </button>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit}>
          {/* Amount in Rupees */}
          <div className="form-group">
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <label className="form-label">Amount (₹ INR)</label>
              <span style={{ fontSize: 11, color: 'var(--emerald-400)', fontFamily: 'var(--font-mono)' }}>
                {paise.toLocaleString()} paise
              </span>
            </div>
            <input
              type="number"
              min="1"
              step="any"
              className="form-input"
              value={rupees}
              onChange={(e) => setRupees(e.target.value)}
              required
            />
          </div>

          {/* Currency */}
          <div className="form-group">
            <label className="form-label">Currency (ISO 4217)</label>
            <input
              type="text"
              maxLength={3}
              className="form-input"
              value={currency}
              onChange={(e) => setCurrency(e.target.value.toUpperCase())}
              required
            />
          </div>

          {/* Description */}
          <div className="form-group">
            <label className="form-label">Description</label>
            <input
              type="text"
              className="form-input"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>

          {/* Metadata Order ID */}
          <div className="form-group">
            <label className="form-label">Order Reference (Metadata)</label>
            <input
              type="text"
              className="form-input form-mono"
              value={orderId}
              onChange={(e) => setOrderId(e.target.value)}
            />
          </div>

          {/* Idempotency Key */}
          <div className="form-group">
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <label className="form-label">Idempotency-Key Header</label>
              <button
                type="button"
                onClick={generateNewKey}
                style={{
                  background: 'none',
                  border: 'none',
                  color: 'var(--emerald-400)',
                  fontSize: 12,
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 4,
                }}
              >
                <RefreshCw size={12} />
                Generate UUID
              </button>
            </div>
            <input
              type="text"
              className="form-input form-mono"
              value={idempotencyKey}
              onChange={(e) => setIdempotencyKey(e.target.value)}
              required
            />
          </div>

          {/* Action button */}
          <div style={{ display: 'flex', gap: 10, marginTop: 24 }}>
            <button
              type="submit"
              className="btn btn-primary"
              disabled={loading || paise <= 0}
              style={{ flex: 1 }}
            >
              <Send size={15} />
              {loading ? 'Processing...' : 'Submit Payment'}
            </button>
            <button type="button" className="btn btn-secondary" onClick={onClose}>
              Cancel
            </button>
          </div>
        </form>

        {/* Live Feedback / Result Callout */}
        {result && (
          <div
            style={{
              marginTop: 20,
              padding: 16,
              borderRadius: 'var(--radius-md)',
              background:
                result.status === 201
                  ? 'rgba(16, 185, 129, 0.1)'
                  : result.status === 200
                  ? 'rgba(56, 189, 248, 0.1)'
                  : 'rgba(244, 63, 94, 0.1)',
              border: `1px solid ${
                result.status === 201
                  ? 'rgba(16, 185, 129, 0.3)'
                  : result.status === 200
                  ? 'rgba(56, 189, 248, 0.3)'
                  : 'rgba(244, 63, 94, 0.3)'
              }`,
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                {result.status === 201 || result.status === 200 ? (
                  <CheckCircle2 size={16} color="var(--emerald-400)" />
                ) : (
                  <AlertCircle size={16} color="#F43F5E" />
                )}
                <span style={{ fontWeight: 600, fontSize: 13, color: '#F8FAFC' }}>
                  HTTP {result.status} {result.status === 201 ? 'Created (New)' : result.status === 200 ? 'OK (Idempotent Replay)' : result.error}
                </span>
              </div>
              <span style={{ fontSize: 11, color: 'var(--text-dim)', fontFamily: 'var(--font-mono)' }}>
                {result.latencyMs}ms
              </span>
            </div>

            {result.data && (
              <div style={{ fontSize: 12, color: 'var(--text-muted)' }}>
                Created ULID: <strong style={{ color: '#F8FAFC', fontFamily: 'var(--font-mono)' }}>{result.data.id}</strong>
                <br />
                Status: <span className={`status-badge ${result.data.status}`} style={{ fontSize: 10, padding: '2px 6px' }}>{result.data.status}</span>
              </div>
            )}

            {result.message && !result.data && (
              <div style={{ fontSize: 12, color: '#F43F5E' }}>{result.message}</div>
            )}
          </div>
        )}
      </div>
    </div>
  );
};
