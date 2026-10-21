import React from 'react';
import { X, Copy, Check } from 'lucide-react';
import { formatINR } from '../lib/api';
import type { Payment } from '../lib/api';

interface PaymentDetailsModalProps {
  payment: Payment | null;
  onClose: () => void;
}

export const PaymentDetailsModal: React.FC<PaymentDetailsModalProps> = ({ payment, onClose }) => {
  const [copied, setCopied] = React.useState(false);

  if (!payment) return null;

  const handleCopyJSON = () => {
    navigator.clipboard.writeText(JSON.stringify(payment, null, 2));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" style={{ maxWidth: 640 }} onClick={(e) => e.stopPropagation()}>
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 20 }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
              <h3 style={{ fontSize: 18, fontWeight: 600, color: '#F8FAFC' }}>Transaction Inspector</h3>
              <span className={`status-badge ${payment.status}`}>{payment.status}</span>
            </div>
            <p style={{ fontSize: 13, color: 'var(--text-dim)', fontFamily: 'var(--font-mono)' }}>
              {payment.id}
            </p>
          </div>
          <button
            onClick={onClose}
            style={{ background: 'none', border: 'none', color: 'var(--text-dim)', cursor: 'pointer' }}
          >
            <X size={20} />
          </button>
        </div>

        {/* Details Grid */}
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: '1fr 1fr',
            gap: 12,
            background: 'rgba(15, 23, 42, 0.6)',
            padding: 16,
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border-subtle)',
            marginBottom: 20,
            fontSize: 13,
          }}
        >
          <div>
            <span style={{ color: 'var(--text-dim)', display: 'block', marginBottom: 2 }}>Amount</span>
            <strong style={{ color: 'var(--emerald-400)', fontSize: 16 }}>{formatINR(payment.amount)}</strong>{' '}
            <span style={{ fontSize: 11, color: 'var(--text-dim)' }}>({payment.amount} paise)</span>
          </div>

          <div>
            <span style={{ color: 'var(--text-dim)', display: 'block', marginBottom: 2 }}>Currency</span>
            <strong style={{ color: '#F8FAFC' }}>{payment.currency}</strong>
          </div>

          <div style={{ gridColumn: 'span 2' }}>
            <span style={{ color: 'var(--text-dim)', display: 'block', marginBottom: 2 }}>Idempotency Key</span>
            <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--text-muted)', wordBreak: 'break-all' }}>
              {payment.idempotency_key}
            </span>
          </div>

          {payment.description && (
            <div style={{ gridColumn: 'span 2' }}>
              <span style={{ color: 'var(--text-dim)', display: 'block', marginBottom: 2 }}>Description</span>
              <span style={{ color: '#F8FAFC' }}>{payment.description}</span>
            </div>
          )}

          <div>
            <span style={{ color: 'var(--text-dim)', display: 'block', marginBottom: 2 }}>Created At</span>
            <span style={{ color: 'var(--text-muted)' }}>{new Date(payment.created_at).toLocaleString()}</span>
          </div>

          <div>
            <span style={{ color: 'var(--text-dim)', display: 'block', marginBottom: 2 }}>Updated At</span>
            <span style={{ color: 'var(--text-muted)' }}>{new Date(payment.updated_at).toLocaleString()}</span>
          </div>

          {payment.provider_ref && (
            <div>
              <span style={{ color: 'var(--text-dim)', display: 'block', marginBottom: 2 }}>Provider Reference</span>
              <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--emerald-400)' }}>{payment.provider_ref}</span>
            </div>
          )}

          {payment.failure_reason && (
            <div>
              <span style={{ color: 'var(--text-dim)', display: 'block', marginBottom: 2 }}>Failure Reason</span>
              <span style={{ color: '#F43F5E' }}>{payment.failure_reason}</span>
            </div>
          )}
        </div>

        {/* Raw JSON View */}
        <div>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
            <span style={{ fontSize: 12, fontWeight: 500, color: 'var(--text-dim)' }}>Raw JSON Response</span>
            <button
              onClick={handleCopyJSON}
              style={{
                background: 'none',
                border: 'none',
                color: copied ? 'var(--emerald-400)' : 'var(--text-dim)',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: 4,
                fontSize: 12,
              }}
            >
              {copied ? <Check size={12} /> : <Copy size={12} />}
              {copied ? 'Copied' : 'Copy JSON'}
            </button>
          </div>
          <pre
            style={{
              background: '#0B0F19',
              border: '1px solid var(--border-subtle)',
              borderRadius: 'var(--radius-md)',
              padding: 12,
              fontFamily: 'var(--font-mono)',
              fontSize: 11,
              color: 'var(--emerald-400)',
              overflowX: 'auto',
              maxHeight: 200,
            }}
          >
            {JSON.stringify(payment, null, 2)}
          </pre>
        </div>

        <div style={{ marginTop: 20, display: 'flex', justifyContent: 'flex-end' }}>
          <button className="btn btn-secondary" onClick={onClose}>
            Close
          </button>
        </div>
      </div>
    </div>
  );
};
