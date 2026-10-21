import React, { useEffect, useState } from 'react';
import { ShieldCheck, Activity, Plus, RefreshCw, Zap } from 'lucide-react';
import { checkGatewayHealth } from '../lib/api';

interface HeaderProps {
  onOpenCreateModal: () => void;
  onOpenIdempotencyLab: () => void;
  onRefreshData: () => void;
  isRefreshing: boolean;
}

export const Header: React.FC<HeaderProps> = ({
  onOpenCreateModal,
  onOpenIdempotencyLab,
  onRefreshData,
  isRefreshing,
}) => {
  const [health, setHealth] = useState<{ ok: boolean; latencyMs: number } | null>(null);

  const pollHealth = async () => {
    const res = await checkGatewayHealth();
    setHealth({ ok: res.ok, latencyMs: res.latencyMs });
  };

  useEffect(() => {
    pollHealth();
    const interval = setInterval(pollHealth, 10000);
    return () => clearInterval(interval);
  }, []);

  return (
    <header className="app-header">
      <div className="header-container">
        {/* Brand */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
          <div
            style={{
              width: 38,
              height: 38,
              borderRadius: 10,
              background: 'linear-gradient(135deg, #10B981 0%, #059669 100%)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              boxShadow: '0 4px 14px rgba(16, 185, 129, 0.35)',
            }}
          >
            <ShieldCheck size={22} color="#FFFFFF" />
          </div>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontSize: 18, fontWeight: 700, letterSpacing: '-0.02em', color: '#F8FAFC' }}>
                PayFlow
              </span>
              <span className="brand-badge">Pine Labs Demo</span>
            </div>
            <div style={{ fontSize: 12, color: 'var(--text-dim)', fontWeight: 400 }}>
              Transaction Engine & Idempotent Gateway
            </div>
          </div>
        </div>

        {/* Health status & CTAs */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          {/* Health Pill */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              padding: '6px 12px',
              borderRadius: 9999,
              background: 'rgba(15, 23, 42, 0.8)',
              border: '1px solid var(--border-subtle)',
              fontSize: 12,
            }}
          >
            <Activity
              size={14}
              color={health?.ok ? 'var(--emerald-400)' : '#F43F5E'}
              style={{ animation: health?.ok ? 'pulse 2s infinite' : 'none' }}
            />
            <span style={{ color: 'var(--text-muted)' }}>Engine:</span>
            <span
              style={{
                color: health?.ok ? 'var(--emerald-400)' : '#F43F5E',
                fontWeight: 600,
              }}
            >
              {health === null ? 'Pinging...' : health.ok ? `Online (${health.latencyMs}ms)` : 'Degraded'}
            </span>
          </div>

          {/* Refresh */}
          <button
            className="btn btn-secondary"
            onClick={onRefreshData}
            disabled={isRefreshing}
            title="Refresh transactions"
            style={{ padding: '8px 12px' }}
          >
            <RefreshCw size={15} style={{ animation: isRefreshing ? 'spin 1s linear infinite' : 'none' }} />
          </button>

          {/* Idempotency Lab CTA */}
          <button
            className="btn btn-secondary"
            onClick={onOpenIdempotencyLab}
            style={{
              borderColor: 'rgba(16, 185, 129, 0.4)',
              background: 'rgba(16, 185, 129, 0.08)',
              color: 'var(--emerald-400)',
            }}
          >
            <Zap size={15} />
            Idempotency Lab
          </button>

          {/* New Payment CTA */}
          <button className="btn btn-primary" onClick={onOpenCreateModal}>
            <Plus size={16} />
            New Payment
          </button>
        </div>
      </div>
    </header>
  );
};
